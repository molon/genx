package gqlx

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/samber/lo"
	"github.com/theplant/relay"
)

func WithSkippedConnection(ctx context.Context) context.Context {
	fields := lo.SliceToMap(graphql.CollectFieldsCtx(ctx, nil), func(f graphql.CollectedField) (string, bool) {
		return f.Name, true
	})
	skip := relay.GetSkip(ctx)
	if _, exists := fields["edges"]; !exists {
		skip.Edges = true
	}
	if _, exists := fields["nodes"]; !exists {
		skip.Nodes = true
	}
	if _, exists := fields["totalCount"]; !exists {
		skip.TotalCount = true
	}
	if _, exists := fields["pageInfo"]; !exists {
		skip.PageInfo = true
	}
	ctx = relay.WithSkip(ctx, skip)
	return ctx
}
