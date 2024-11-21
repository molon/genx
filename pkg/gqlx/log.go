package gqlx

import (
	"context"
	"log"

	"github.com/99designs/gqlgen/graphql"
)

type LoggingInterceptor struct{}

var _ interface {
	graphql.HandlerExtension
	graphql.ResponseInterceptor
} = &LoggingInterceptor{}

func (v *LoggingInterceptor) ExtensionName() string {
	return "Logging"
}

func (v *LoggingInterceptor) Validate(graphql.ExecutableSchema) error {
	return nil
}

func (v *LoggingInterceptor) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	resp := next(ctx)
	if resp != nil && len(resp.Errors) > 0 {
		for _, err := range resp.Errors {
			log.Printf("[GraphQL Error] Message: %s, Path: %v, Extensions: %v", err.Message, err.Path, err.Extensions)
		}
	}
	return resp
}
