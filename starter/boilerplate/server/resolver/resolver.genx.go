package resolver

import (
	"context"
	"database/sql/driver"
	"net/http"

	"github.com/molon/genx/pkg/gqlx"
	"github.com/molon/genx/starter/boilerplate/server/model"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vikstrous/dataloadgen"
	"gorm.io/gorm"
)

type Resolver struct {
	db      *gorm.DB
	Company *CompanyResolver
	Task    *TaskResolver
	User    *UserResolver
}

func New(db *gorm.DB) *Resolver {
	r := &Resolver{db: db}
	r.Company = NewCompanyResolver(r)
	r.Task = NewTaskResolver(r)
	r.User = NewUserResolver(r)
	return r
}

type Loader struct {
	Company *dataloadgen.Loader[string, *model.Company]
	Task    *dataloadgen.Loader[string, *model.Task]
	User    *dataloadgen.Loader[string, *model.User]
}

type (
	ctxKeyDB     struct{}
	ctxKeyTx     struct{}
	ctxKeyLoader struct{}
)

func (r *Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// TODO: loader 放到这是不是不太合适呢，因为一个请求可能有多个 query 和 mutation ，对于 query 的话返回值相同可以接受，那么对于 mutation 的话呢？
		loader := &Loader{
			Company: r.Company.NewLoader(),
			Task:    r.Task.NewLoader(),
			User:    r.User.NewLoader(),
		}
		ctx := context.WithValue(req.Context(), ctxKeyLoader{}, loader)
		// TODO: 如果是 mutaion 的话，这一句实际上没啥意义了，所以事务那段可以在这里搞吗？
		ctx = context.WithValue(ctx, ctxKeyDB{}, r.db.WithContext(ctx))
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func (r *Resolver) Loader(ctx context.Context) *Loader {
	loader, _ := ctx.Value(ctxKeyLoader{}).(*Loader)
	if loader == nil {
		panic(errors.New("loader not found in context"))
	}
	return loader
}

func (r *Resolver) DB(ctx context.Context) *gorm.DB {
	db, _ := ctx.Value(ctxKeyTx{}).(*gorm.DB)
	if db == nil {
		db, _ = ctx.Value(ctxKeyDB{}).(*gorm.DB)
	}
	if db == nil {
		panic(errors.New("db not found in context"))
	}
	return db
}

func (r *Resolver) OpenTx(ctx context.Context, op *ast.OperationDefinition) (context.Context, driver.Tx, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return ctx, nil, errors.Wrap(tx.Error, "failed to begin transaction") // TODO: gqlerror?
	}
	ctx = context.WithValue(ctx, ctxKeyTx{}, tx)
	return ctx, gqlx.Tx(
		func() error { return tx.Commit().Error },
		func() error { return tx.Rollback().Error },
	), nil
}

func generateID() string {
	return xid.New().String()
}
