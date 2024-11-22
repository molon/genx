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

type CompanyResolver struct {
	*Resolver
	pagination relay.Pagination[*model.Company]
}

func NewCompanyResolver(r *Resolver) *CompanyResolver {
	c := &CompanyResolver{Resolver: r}
	c.initPagination()
	return c
}

func (c *CompanyResolver) initPagination() {
	c.pagination = relay.New(
		cursor.Base64(func(ctx context.Context, req *relay.ApplyCursorsRequest) (*relay.ApplyCursorsResponse[*model.Company], error) {
			// TODO: 做一下 select 处理？并且对于 keyset 的情况一定要包含最终的 order by 字段
			return gormrelay.NewKeysetAdapter[*model.Company](c.DB(ctx))(ctx, req)
		}),
		relay.EnsureLimits[*model.Company](100, 10),
		relay.EnsurePrimaryOrderBy[*model.Company](
			relay.OrderBy{Field: "CreatedAt", Desc: false},
		),
	)
}

func (c *CompanyResolver) batchRead(ctx context.Context, ids []string) ([]*model.Company, []error) {
	if len(ids) == 0 {
		return []*model.Company{}, nil
	}

	db := c.DB(ctx)

	var companies []*model.Company
	if err := db.Find(&companies, "id IN ?", ids).Error; err != nil {
		return nil, []error{errors.Wrap(err, "failed to find companies")}
	}

	idToCompany := make(map[string]*model.Company, len(companies))
	for _, company := range companies {
		idToCompany[company.ID] = company
	}

	result := make([]*model.Company, len(ids))
	for i, id := range ids {
		result[i] = idToCompany[id]
	}
	return result, nil
}

func (c *CompanyResolver) NewLoader() *dataloadgen.Loader[string, *model.Company] {
	return dataloadgen.NewLoader(
		c.batchRead,
		dataloadgen.WithBatchCapacity(100),
		dataloadgen.WithWait(5*time.Millisecond),
	)
}

func (c *CompanyResolver) Loader(ctx context.Context) *dataloadgen.Loader[string, *model.Company] {
	return c.Resolver.Loader(ctx).Company
}

func (c *CompanyResolver) Get(ctx context.Context, id *string) (*model.Company, error) {
	if id == nil {
		return nil, nil
	}
	return c.Loader(ctx).Load(ctx, *id)
}

func (c *CompanyResolver) List(ctx context.Context, after *string, first *int, before *string, last *int, _ *model.CompanyFilter, orderBy []*model.CompanyOrder) (*model.CompanyConnection, error) {
	return c.pagination.Paginate(
		relay.WithNodeProcessor(
			gqlx.WithSkippedConnection(ctx),
			func(node *model.Company) *model.Company {
				// TODO: 如果某 id 对应的已经在 cache 里了，那之前从 cache 里取出来的数据使用的地方貌似不一定会和这里一致吧。貌似应该让 dataloader 支持先直接取 cache 。
				c.Loader(ctx).Prime(node.ID, node)
				return node
			},
		),
		&relay.PaginateRequest[*model.Company]{
			First: first, After: after, Last: last, Before: before,
			OrderBys: lo.Map(orderBy, func(order *model.CompanyOrder, _ int) relay.OrderBy {
				return relay.OrderBy{
					Field: lo.PascalCase(order.Field.String()),
					Desc:  order.Direction == model.OrderDirectionDesc,
				}
			}),
		},
	)
}

func (c *CompanyResolver) Employees(ctx context.Context, company *model.Company, after *string, first *int, before *string, last *int, filterBy *model.UserFilter, orderBy []*model.UserOrder) (*relay.Connection[*model.User], error) {
	// TODO: Need to cooperate with the corresponding one to one
	// filterBy.Company = &model.CompanyFilter{
	// 	ID: &model.IDFilter{Equals: &company.ID},
	// }
	return c.Resolver.User.List(ctx, after, first, before, last, filterBy, orderBy)
}

func (c *CompanyResolver) new(_ context.Context, input model.CreateCompanyInput) *model.Company {
	return &model.Company{
		ID:          generateID(),
		Name:        input.Name,
		Description: input.Description,
	}
}

func (c *CompanyResolver) create(ctx context.Context, company *model.Company) error {
	db := c.DB(ctx)
	if err := db.Create(company).Error; err != nil {
		return errors.Wrap(err, "failed to create company")
	}
	c.Loader(ctx).Prime(company.ID, company)
	return nil
}

func (c *CompanyResolver) Create(ctx context.Context, input model.CreateCompanyInput) (*model.CreateCompanyPayload, error) {
	// TODO: should check permission

	company := c.new(ctx, input)

	if err := c.validate(ctx, company); err != nil {
		return nil, err
	}

	if err := c.create(ctx, company); err != nil {
		return nil, err
	}

	return &model.CreateCompanyPayload{
		ClientMutationID: input.ClientMutationID,
		Company:          company,
	}, nil
}

func (c *CompanyResolver) unmarshal(_ context.Context, company *model.Company, input model.UpdateCompanyInput, inputFields map[string]any) error {
	for field := range inputFields {
		switch field {
		case "name":
			company.Name = *input.Name
		case "description":
			company.Description = input.Description
		}
	}
	return nil
}

func (c *CompanyResolver) update(ctx context.Context, company *model.Company) error {
	db := c.DB(ctx)
	if err := db.Save(company).Error; err != nil {
		return errors.Wrap(err, "failed to update company")
	}
	c.Loader(ctx).Prime(company.ID, company)
	return nil
}

func (c *CompanyResolver) Update(ctx context.Context, input model.UpdateCompanyInput, inputFields map[string]any) (*model.UpdateCompanyPayload, error) {
	// TODO: should check permission

	// TODO: 还是要好好思考下为什么不能直接通过 dataloader 取出来的数据直接修改，而是要重新查一遍，难道是因为多个 mutation 的情况？
	company, err := c.first(ctx, input.CompanyID)
	if err != nil {
		return nil, err
	}

	if err := c.unmarshal(ctx, company, input, inputFields); err != nil {
		return nil, err
	}

	if err := c.validate(ctx, company); err != nil {
		return nil, err
	}

	if err := c.update(ctx, company); err != nil {
		return nil, err
	}

	// TODO: 需要测试，这里返回之后的嵌套后续 resolver 会先执行，然后再执行另外一个 mutation 请求还是如何。
	// TODO: 或许应该对于 mutation 操作应该单独的 dataloader ，而 query 则另说？

	return &model.UpdateCompanyPayload{
		ClientMutationID: input.ClientMutationID,
		Company:          company,
	}, nil
}

func (c *CompanyResolver) delete(ctx context.Context, company *model.Company) error {
	db := c.DB(ctx)
	if err := db.Delete(&company).Error; err != nil {
		return errors.Wrap(err, "failed to delete company")
	}
	c.Loader(ctx).Clear(company.ID)
	return nil
}

func (c *CompanyResolver) Delete(ctx context.Context, input model.DeleteCompanyInput) (*model.DeleteCompanyPayload, error) {
	// TODO: should check permission

	company, err := c.first(ctx, input.CompanyID)
	if err != nil {
		return nil, err
	}

	if err := c.delete(ctx, company); err != nil {
		return nil, err
	}

	return &model.DeleteCompanyPayload{
		ClientMutationID: input.ClientMutationID,
		Company:          company,
	}, nil
}

func (c *CompanyResolver) first(ctx context.Context, id string) (*model.Company, error) {
	db := c.DB(ctx)

	var company model.Company
	if err := db.First(&company, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.Wrap(err, "company not found")
		}
		return nil, errors.Wrap(err, "failed to fetch company")
	}

	return &company, nil
}

func (c *CompanyResolver) validate(ctx context.Context, company *model.Company) error {
	// TODO: should zod validate
	// TODO: Add validation logic if needed
	return nil
}

func (c *CompanyResolver) ViewerPermission(ctx context.Context, company *model.Company) (*model.CompanyViewerPermission, error) {
	// TODO: ladon
	return &model.CompanyViewerPermission{
		CanCreate: true,
		CanUpdate: true,
		CanDelete: true,
	}, nil
}
