package relayext

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/plugin"
	"github.com/molon/genx/extension/gqlgenext"
	"github.com/samber/lo"
)

var _ gqlgenext.WithGQLPlugins = (*Extension)(nil)

func (e *Extension) GQLPlugins() []plugin.Plugin {
	return []plugin.Plugin{e.gqlResolverImpl}
}

type gqlResolverImplementer struct {
	data *Data
}

func (i *gqlResolverImplementer) Name() string {
	return "RelayGQLResolverImplementer"
}

var reMutation = regexp.MustCompile(`^(create|update|delete)([A-Z]\w+)$`)

func (i *gqlResolverImplementer) Implement(body string, field *codegen.Field) string {
	// return "panic(\"implementer implemented me\")"
	if IsMethodField(field.FieldDefinition) {
		if field.Object.Name == "Mutation" {
			if len(field.Args) != 1 {
				return body
			}
			matches := reMutation.FindStringSubmatch(field.Name)
			if len(matches) <= 2 {
				return body
			}
			nodeName := matches[2]
			node := i.data.GetNode(nodeName)
			if node == nil {
				return body
			}
			op := lo.PascalCase(matches[1])
			if op == "Update" {
				return fmt.Sprintf(`inputFields, _ := gqlx.CollectArgumentFields(ctx)["input"].(map[string]any)
					return r.Resolver.%s.%s(ctx, %s, inputFields)`,
					lo.PascalCase(node.Name), op, field.Args[0].VarName,
				)
			}
			return fmt.Sprintf("return r.Resolver.%s.%s(ctx, %s)", lo.PascalCase(node.Name), op, field.Args[0].VarName)
		}

		if field.Object.Name == "Query" {
			typRef := field.TypeReference
			if typRef == nil {
				return body
			}
			if !strings.HasSuffix(typRef.Definition.Name, "Connection") {
				return body
			}
			nodeName := strings.TrimSuffix(typRef.Definition.Name, "Connection")
			node := i.data.GetNode(nodeName)
			if node == nil {
				return body
			}
			args := lo.Map(field.Args, func(arg *codegen.FieldArgument, _ int) string {
				return ", " + arg.VarName
			})
			return fmt.Sprintf("return r.Resolver.%s.List(ctx%s)", nodeName, strings.Join(args, ""))
		}
	}

	// node field resolver
	node := i.data.GetNode(field.Object.Name)
	if node == nil {
		return body
	}
	args := lo.Map(field.Args, func(arg *codegen.FieldArgument, _ int) string {
		return ", " + arg.VarName
	})
	return fmt.Sprintf("return r.Resolver.%s.%s(ctx, obj%s)", field.Object.Name, lo.PascalCase(field.Name), strings.Join(args, ""))
}
