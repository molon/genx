package resolver

import (
	"context"
	"time"

	"github.com/molon/genx/pkg/gqlx"
	"github.com/molon/genx/starter/boilerplate/server/model"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/theplant/relay"
	"github.com/theplant/relay/cursor"
	"github.com/theplant/relay/gormrelay"
	"github.com/vikstrous/dataloadgen"
	"gorm.io/gorm"
)

type UserResolver struct {
	*Resolver
	pagination relay.Pagination[*model.User]
}

func NewUserResolver(r *Resolver) *UserResolver {
	c := &UserResolver{Resolver: r}
	c.initPagination()
	return c
}

func (c *UserResolver) initPagination() {
	c.pagination = relay.New(
		cursor.Base64(func(ctx context.Context, req *relay.ApplyCursorsRequest) (*relay.ApplyCursorsResponse[*model.User], error) {
			// TODO: 做一下 select 处理？并且对于 keyset 的情况一定要包含最终的 order by 字段
			return gormrelay.NewKeysetAdapter[*model.User](c.DB(ctx))(ctx, req)
		}),
		relay.EnsureLimits[*model.User](100, 10),
		relay.EnsurePrimaryOrderBy[*model.User](
			relay.OrderBy{Field: "CreatedAt", Desc: false},
		),
	)
}

func (c *UserResolver) batchRead(ctx context.Context, ids []string) ([]*model.User, []error) {
	if len(ids) == 0 {
		return []*model.User{}, nil
	}

	db := c.DB(ctx)

	var users []*model.User
	if err := db.Find(&users, "id IN ?", ids).Error; err != nil {
		return nil, []error{errors.Wrap(err, "failed to find users")}
	}

	idToUser := make(map[string]*model.User, len(users))
	for _, user := range users {
		idToUser[user.ID] = user
	}

	result := make([]*model.User, len(ids))
	for i, id := range ids {
		result[i] = idToUser[id]
	}
	return result, nil
}

func (c *UserResolver) NewLoader() *dataloadgen.Loader[string, *model.User] {
	return dataloadgen.NewLoader(
		c.batchRead,
		dataloadgen.WithBatchCapacity(100),
		dataloadgen.WithWait(5*time.Millisecond),
	)
}

func (c *UserResolver) Loader(ctx context.Context) *dataloadgen.Loader[string, *model.User] {
	return c.Resolver.Loader(ctx).User
}

func (c *UserResolver) Get(ctx context.Context, id *string) (*model.User, error) {
	if id == nil {
		return nil, nil
	}
	return c.Loader(ctx).Load(ctx, *id)
}

func (c *UserResolver) List(ctx context.Context, after *string, first *int, before *string, last *int, _ *model.UserFilter, orderBy []*model.UserOrder) (*model.UserConnection, error) {
	return c.pagination.Paginate(
		relay.WithNodeProcessor(
			gqlx.WithSkippedConnection(ctx),
			func(node *model.User) *model.User {
				// TODO: 如果某 id 对应的已经在 cache 里了，那之前从 cache 里取出来的数据使用的地方貌似不一定会和这里一致吧。貌似应该让 dataloader 支持先直接取 cache 。
				c.Loader(ctx).Prime(node.ID, node)
				return node
			},
		),
		&relay.PaginateRequest[*model.User]{
			First: first, After: after, Last: last, Before: before,
			OrderBys: lo.Map(orderBy, func(order *model.UserOrder, _ int) relay.OrderBy {
				return relay.OrderBy{
					Field: lo.PascalCase(order.Field.String()),
					Desc:  order.Direction == model.OrderDirectionDesc,
				}
			}),
		},
	)
}

func (c *UserResolver) Company(ctx context.Context, user *model.User) (*model.Company, error) {
	return c.Resolver.Company.Get(ctx, &user.CompanyID)
}

func (c *UserResolver) Tasks(ctx context.Context, user *model.User, after *string, first *int, before *string, last *int, filterBy *model.TaskFilter, orderBy []*model.TaskOrder) (*relay.Connection[*model.Task], error) {
	// TODO: Need to cooperate with the corresponding one to one
	// filterBy.User = &model.UserFilter{
	// 	ID: &model.IDFilter{Equals: &user.ID},
	// }
	return c.Resolver.Task.List(ctx, after, first, before, last, filterBy, orderBy)
}

func (c *UserResolver) new(_ context.Context, input model.CreateUserInput) *model.User {
	return &model.User{
		ID:          generateID(),
		Name:        input.Name,
		Description: input.Description,
		Age:         input.Age,
		CompanyID:   input.CompanyID,
	}
}

func (c *UserResolver) create(ctx context.Context, user *model.User) error {
	db := c.DB(ctx)
	if err := db.Create(user).Error; err != nil {
		return errors.Wrap(err, "failed to create user")
	}
	c.Loader(ctx).Prime(user.ID, user)
	return nil
}

func (c *UserResolver) Create(ctx context.Context, input model.CreateUserInput) (*model.CreateUserPayload, error) {
	// TODO: should check permission

	user := c.new(ctx, input)

	if err := c.validate(ctx, user); err != nil {
		return nil, err
	}

	if err := c.create(ctx, user); err != nil {
		return nil, err
	}

	return &model.CreateUserPayload{
		ClientMutationID: input.ClientMutationID,
		User:             user,
	}, nil
}

func (c *UserResolver) unmarshal(_ context.Context, user *model.User, input model.UpdateUserInput, inputFields map[string]any) error {
	for field := range inputFields {
		switch field {
		case "name":
			user.Name = *input.Name
		case "description":
			user.Description = input.Description
		case "age":
			user.Age = *input.Age
		case "companyId":
			user.CompanyID = *input.CompanyID
		}
	}
	return nil
}

func (c *UserResolver) update(ctx context.Context, user *model.User) error {
	db := c.DB(ctx)
	if err := db.Save(user).Error; err != nil {
		return errors.Wrap(err, "failed to update user")
	}
	c.Loader(ctx).Prime(user.ID, user)
	return nil
}

func (c *UserResolver) Update(ctx context.Context, input model.UpdateUserInput, inputFields map[string]any) (*model.UpdateUserPayload, error) {
	// TODO: should check permission

	// TODO: 还是要好好思考下为什么不能直接通过 dataloader 取出来的数据直接修改，而是要重新查一遍，难道是因为多个 mutation 的情况？
	user, err := c.first(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	if err := c.unmarshal(ctx, user, input, inputFields); err != nil {
		return nil, err
	}

	if err := c.validate(ctx, user); err != nil {
		return nil, err
	}

	if err := c.update(ctx, user); err != nil {
		return nil, err
	}

	// TODO: 需要测试，这里返回之后的嵌套后续 resolver 会先执行，然后再执行另外一个 mutation 请求还是如何。
	// TODO: 或许应该对于 mutation 操作应该单独的 dataloader ，而 query 则另说？

	return &model.UpdateUserPayload{
		ClientMutationID: input.ClientMutationID,
		User:             user,
	}, nil
}

func (c *UserResolver) delete(ctx context.Context, user *model.User) error {
	db := c.DB(ctx)
	if err := db.Delete(&user).Error; err != nil {
		return errors.Wrap(err, "failed to delete user")
	}
	c.Loader(ctx).Clear(user.ID)
	return nil
}

func (c *UserResolver) Delete(ctx context.Context, input model.DeleteUserInput) (*model.DeleteUserPayload, error) {
	// TODO: should check permission

	user, err := c.first(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	if err := c.delete(ctx, user); err != nil {
		return nil, err
	}

	return &model.DeleteUserPayload{
		ClientMutationID: input.ClientMutationID,
		User:             user,
	}, nil
}

func (c *UserResolver) first(ctx context.Context, id string) (*model.User, error) {
	db := c.DB(ctx)

	var user model.User
	if err := db.First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.Wrap(err, "user not found")
		}
		return nil, errors.Wrap(err, "failed to fetch user")
	}

	return &user, nil
}

func (c *UserResolver) validate(ctx context.Context, user *model.User) error {
	// TODO: should zod validate
	// TODO: Add validation logic if needed
	if user.CompanyID != "" {
		company, err := c.Resolver.Company.Get(ctx, &user.CompanyID)
		// TODO: 这里貌似应该从 db 里查才 OK ？
		if err != nil {
			return err
		}
		if company == nil {
			return errors.New("company not found")
		}
	}
	return nil
}

func (c *UserResolver) ViewerPermission(ctx context.Context, user *model.User) (*model.UserViewerPermission, error) {
	// TODO: ladon
	return &model.UserViewerPermission{
		CanCreate: true,
		CanUpdate: true,
		CanDelete: true,
	}, nil
}
