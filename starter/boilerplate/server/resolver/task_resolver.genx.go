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

type TaskResolver struct {
	*Resolver
	pagination relay.Pagination[*model.Task]
}

func NewTaskResolver(r *Resolver) *TaskResolver {
	c := &TaskResolver{Resolver: r}
	c.initPagination()
	return c
}

func (c *TaskResolver) initPagination() {
	c.pagination = relay.New(
		cursor.Base64(func(ctx context.Context, req *relay.ApplyCursorsRequest) (*relay.ApplyCursorsResponse[*model.Task], error) {
			// TODO: 做一下 select 处理？并且对于 keyset 的情况一定要包含最终的 order by 字段
			return gormrelay.NewKeysetAdapter[*model.Task](c.DB(ctx))(ctx, req)
		}),
		relay.EnsureLimits[*model.Task](100, 10),
		relay.EnsurePrimaryOrderBy[*model.Task](
			relay.OrderBy{Field: "CreatedAt", Desc: false},
		),
	)
}

func (c *TaskResolver) batchRead(ctx context.Context, ids []string) ([]*model.Task, []error) {
	if len(ids) == 0 {
		return []*model.Task{}, nil
	}

	db := c.DB(ctx)

	var tasks []*model.Task
	if err := db.Find(&tasks, "id IN ?", ids).Error; err != nil {
		return nil, []error{errors.Wrap(err, "failed to find tasks")}
	}
	return tasks, nil
}

func (c *TaskResolver) NewLoader() *dataloadgen.Loader[string, *model.Task] {
	return dataloadgen.NewLoader(
		c.batchRead,
		dataloadgen.WithBatchCapacity(100),
		dataloadgen.WithWait(5*time.Millisecond),
	)
}

func (c *TaskResolver) Loader(ctx context.Context) *dataloadgen.Loader[string, *model.Task] {
	return c.Resolver.Loader(ctx).Task
}

func (c *TaskResolver) Get(ctx context.Context, id *string) (*model.Task, error) {
	if id == nil {
		// TODO: should return nil or error?
		return nil, nil
	}
	return c.Loader(ctx).Load(ctx, *id)
}

func (c *TaskResolver) List(ctx context.Context, after *string, first *int, before *string, last *int, _ *model.TaskFilter, orderBy []*model.TaskOrder) (*model.TaskConnection, error) {
	return c.pagination.Paginate(
		relay.WithNodeProcessor(
			gqlx.WithSkippedConnection(ctx),
			func(node *model.Task) *model.Task {
				// TODO: 如果某 id 对应的已经在 cache 里了，那之前从 cache 里取出来的数据使用的地方貌似不一定会和这里一致吧。貌似应该让 dataloader 支持先直接取 cache 。
				c.Loader(ctx).Prime(node.ID, node)
				return node
			},
		),
		&relay.PaginateRequest[*model.Task]{
			First: first, After: after, Last: last, Before: before,
			OrderBys: lo.Map(orderBy, func(order *model.TaskOrder, _ int) relay.OrderBy {
				return relay.OrderBy{
					Field: lo.PascalCase(order.Field.String()),
					Desc:  order.Direction == model.OrderDirectionDesc,
				}
			}),
		},
	)
}

func (c *TaskResolver) Assignee(ctx context.Context, task *model.Task) (*model.User, error) {
	return c.Resolver.User.Get(ctx, task.AssigneeID)
}

func (c *TaskResolver) new(_ context.Context, input model.CreateTaskInput) *model.Task {
	return &model.Task{
		ID:          generateID(),
		Title:       input.Title,
		Description: input.Description,
		Status:      input.Status,
		AssigneeID:  input.AssigneeID,
	}
}

func (c *TaskResolver) create(ctx context.Context, task *model.Task) error {
	db := c.DB(ctx)
	if err := db.Create(task).Error; err != nil {
		return errors.Wrap(err, "failed to create task")
	}
	c.Loader(ctx).Prime(task.ID, task)
	return nil
}

func (c *TaskResolver) Create(ctx context.Context, input model.CreateTaskInput) (*model.CreateTaskPayload, error) {
	// TODO: should check permission

	task := c.new(ctx, input)

	if err := c.validate(ctx, task); err != nil {
		return nil, err
	}

	if err := c.create(ctx, task); err != nil {
		return nil, err
	}

	return &model.CreateTaskPayload{
		ClientMutationID: input.ClientMutationID,
		Task:             task,
	}, nil
}

func (c *TaskResolver) unmarshal(_ context.Context, task *model.Task, input model.UpdateTaskInput, inputFields map[string]any) error {
	for field := range inputFields {
		switch field {
		case "title":
			task.Title = *input.Title
		case "description":
			task.Description = input.Description
		case "status":
			task.Status = *input.Status
		case "assigneeId":
			task.AssigneeID = input.AssigneeID
		}
	}
	return nil
}

func (c *TaskResolver) update(ctx context.Context, task *model.Task) error {
	db := c.DB(ctx)
	if err := db.Save(task).Error; err != nil {
		return errors.Wrap(err, "failed to update task")
	}
	c.Loader(ctx).Prime(task.ID, task)
	return nil
}

func (c *TaskResolver) Update(ctx context.Context, input model.UpdateTaskInput, inputFields map[string]any) (*model.UpdateTaskPayload, error) {
	// TODO: should check permission

	// TODO: 还是要好好思考下为什么不能直接通过 dataloader 取出来的数据直接修改，而是要重新查一遍，难道是因为多个 mutation 的情况？
	task, err := c.first(ctx, input.TaskID)
	if err != nil {
		return nil, err
	}

	if err := c.unmarshal(ctx, task, input, inputFields); err != nil {
		return nil, err
	}

	if err := c.validate(ctx, task); err != nil {
		return nil, err
	}

	if err := c.update(ctx, task); err != nil {
		return nil, err
	}

	// TODO: 需要测试，这里返回之后的嵌套后续 resolver 会先执行，然后再执行另外一个 mutation 请求还是如何。
	// TODO: 或许应该对于 mutation 操作应该单独的 dataloader ，而 query 则另说？

	return &model.UpdateTaskPayload{
		ClientMutationID: input.ClientMutationID,
		Task:             task,
	}, nil
}

func (c *TaskResolver) delete(ctx context.Context, task *model.Task) error {
	db := c.DB(ctx)
	if err := db.Delete(&task).Error; err != nil {
		return errors.Wrap(err, "failed to delete task")
	}
	c.Loader(ctx).Clear(task.ID)
	return nil
}

func (c *TaskResolver) Delete(ctx context.Context, input model.DeleteTaskInput) (*model.DeleteTaskPayload, error) {
	// TODO: should check permission

	task, err := c.first(ctx, input.TaskID)
	if err != nil {
		return nil, err
	}

	if err := c.delete(ctx, task); err != nil {
		return nil, err
	}

	return &model.DeleteTaskPayload{
		ClientMutationID: input.ClientMutationID,
		Task:             task,
	}, nil
}

func (c *TaskResolver) first(ctx context.Context, id string) (*model.Task, error) {
	db := c.DB(ctx)

	var task model.Task
	if err := db.First(&task, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.Wrap(err, "task not found")
		}
		return nil, errors.Wrap(err, "failed to fetch task")
	}

	return &task, nil
}

func (c *TaskResolver) validate(ctx context.Context, task *model.Task) error {
	// TODO: should zod validate
	// TODO: Add validation logic if needed
	if task.AssigneeID != nil {
		_, err := c.Resolver.User.Get(ctx, task.AssigneeID)
		// TODO: 这里貌似应该从 db 里查才 OK ？
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *TaskResolver) ViewerPermission(ctx context.Context, task *model.Task) (*model.TaskViewerPermission, error) {
	// TODO: ladon
	return &model.TaskViewerPermission{
		CanCreate: true,
		CanUpdate: true,
		CanDelete: true,
	}, nil
}
