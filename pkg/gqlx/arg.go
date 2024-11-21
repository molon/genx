package gqlx

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

// TODO: https://github.com/99designs/gqlgen/issues/866#issuecomment-737684323 回头调研下这种写法是不是更好

// CollectArgumentFields collects the arguments of the current field and returns them as a map[string]any, including nested fields
func CollectArgumentFields(ctx context.Context) map[string]any {
	argumentFields := map[string]any{}
	fieldCtx := graphql.GetFieldContext(ctx)
	if fieldCtx == nil {
		return argumentFields
	}

	// Loop through the arguments of the current field
	for _, arg := range fieldCtx.Field.Arguments {
		argMap := map[string]any{}
		argumentFields[arg.Name] = argMap
		processValueChildren(arg.Value.Children, argMap)
	}

	return argumentFields
}

// Recursive function to process nested fields and build a map[string]any
func processValueChildren(children []*ast.ChildValue, parentMap map[string]any) {
	for _, child := range children {
		if len(child.Value.Children) > 0 {
			// If there are nested fields, create a new map for the child
			childMap := map[string]any{}
			parentMap[child.Name] = childMap
			processValueChildren(child.Value.Children, childMap)
		} else {
			// Otherwise, just mark the field as present
			parentMap[child.Name] = nil
		}
	}
}
