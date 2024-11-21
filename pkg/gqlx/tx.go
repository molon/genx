package gqlx

import (
	"context"
	"database/sql/driver"
	"slices"
	"sync"

	"github.com/pkg/errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type tx struct {
	commit   func() error
	rollback func() error
}

var _ driver.Tx = &tx{}

func Tx(commit func() error, rollback func() error) *tx {
	return &tx{
		commit:   commit,
		rollback: rollback,
	}
}

func (tx *tx) Commit() error {
	return tx.commit()
}

func (tx *tx) Rollback() error {
	return tx.rollback()
}

type TxOpener interface {
	OpenTx(ctx context.Context, op *ast.OperationDefinition) (context.Context, driver.Tx, error)
}

type TxOpenerFunc func(ctx context.Context, op *ast.OperationDefinition) (context.Context, driver.Tx, error)

func (f TxOpenerFunc) OpenTx(ctx context.Context, op *ast.OperationDefinition) (context.Context, driver.Tx, error) {
	return f(ctx, op)
}

type TxSkipFunc func(op *ast.OperationDefinition) bool

type TxMutator struct {
	TxOpener
	TxSkipFunc
}

func TxSkipOperations(names ...string) TxSkipFunc {
	return func(op *ast.OperationDefinition) bool {
		return slices.Contains(names, op.Name)
	}
}

func TxSkipIfHasFields(names ...string) TxSkipFunc {
	return func(op *ast.OperationDefinition) bool {
		return slices.ContainsFunc(op.SelectionSet, func(s ast.Selection) bool {
			f, ok := s.(*ast.Field)
			return ok && slices.Contains(names, f.Name)
		})
	}
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
	graphql.ResponseInterceptor
} = TxMutator{}

func (TxMutator) ExtensionName() string {
	return "TxMutator"
}

func (t TxMutator) Validate(graphql.ExecutableSchema) error {
	if t.TxOpener == nil {
		return errors.New("tx_mutator: tx opener is nil")
	}
	return nil
}

func (t TxMutator) MutateOperationContext(_ context.Context, oc *graphql.OperationContext) *gqlerror.Error {
	if !t.skipTx(oc.Operation) {
		previous := oc.ResolverMiddleware
		var mu sync.Mutex
		oc.ResolverMiddleware = func(ctx context.Context, next graphql.Resolver) (interface{}, error) {
			mu.Lock()
			defer mu.Unlock()
			// TODO: 这里如果这样写的话，是不是会影响到后续 query 的并行呢？需要测试。
			return previous(ctx, next)
		}
	}
	return nil
}

func (t TxMutator) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	// TODO: 如果调用者直接调用了两个 mutation， 它们会在一个事务里吗？
	op := graphql.GetOperationContext(ctx).Operation
	if t.skipTx(op) {
		return next(ctx)
	}

	ctx, tx, err := t.OpenTx(ctx, op)
	if err != nil {
		return graphql.ErrorResponse(ctx, "cannot create transaction: %s", err.Error())
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	rsp := next(ctx)
	if len(rsp.Errors) > 0 {
		_ = tx.Rollback()
		return rsp
	}

	if err := tx.Commit(); err != nil {
		return graphql.ErrorResponse(ctx, "cannot commit transaction: %s", err.Error())
	}
	return rsp
}

func (t TxMutator) skipTx(op *ast.OperationDefinition) bool {
	return op == nil || op.Operation != ast.Mutation || (t.TxSkipFunc != nil && t.TxSkipFunc(op))
}
