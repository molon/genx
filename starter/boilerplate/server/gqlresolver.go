package server

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/molon/genx/pkg/gqlx"
	"github.com/molon/genx/starter/boilerplate/server/exec"
	"github.com/molon/genx/starter/boilerplate/server/resolver"
	"gorm.io/gorm"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type GQLResolver struct {
	*resolver.Resolver
}

func NewGQLHandler(db *gorm.DB) http.Handler {
	resolver := resolver.New(db)
	srv := handler.NewDefaultServer(
		exec.NewExecutableSchema(
			exec.Config{
				Resolvers: &GQLResolver{
					Resolver: resolver,
				},
			},
		),
	)
	srv.Use(gqlx.TxMutator{TxOpener: resolver})
	return resolver.Middleware(srv)
}
