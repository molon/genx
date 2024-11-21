package relayext

import (
	"context"
	_ "embed"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/huandu/go-clone"
	"github.com/jinzhu/inflection"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

//go:embed embed/prelude.relay.genx.graphql
var prelude string

const directiveNode = "node"

var reservedFields = map[string]struct{}{
	"viewerPermission": {},
}

type enhanceSchemaResult struct {
	Document *ast.SchemaDocument
	Nodes    map[string]*ast.Definition
}

func enhanceSchema(_ context.Context, doc *ast.SchemaDocument) (*enhanceSchemaResult, error) {
	sd, err := parser.ParseSchemas(&ast.Source{
		Name:  "prelude.relay.genx.graphql",
		Input: prelude,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse prelude schema")
	}
	sd.Merge(doc)

	r := &enhanceSchemaResult{
		Document: sd,
		Nodes:    make(map[string]*ast.Definition),
	}

	var defs, exts ast.DefinitionList
	for _, def := range sd.Definitions {
		defs = append(defs, def)

		if def.Kind != ast.Object {
			continue
		}

		if !directiveExists(def, directiveNode) {
			continue
		}

		r.Nodes[def.Name] = def

		ensureBuiltInNodeFields(def)
		if err := ensureFieldConnections(sd, def); err != nil {
			return nil, err
		}
		exts = append(exts, ensureQuery(sd, def)...)
		defs = append(defs, ensureConnectionTypes(sd, def)...)
		defs = append(defs, ensureFilter(sd, def)...)
		defs = append(defs, ensureOrderTypes(sd, def)...)
		exts = append(exts, ensureMutation(sd, def)...)
		defs = append(defs, ensureMutationTypes(sd, def)...)
		defs = append(defs, ensureViewerPermission(sd, def)...)
	}

	// TODO: 需要处理完全没有设置 node 标记的情况
	// TODO: 需要为 node 设置全局配置
	// TODO: 针对于 prelude 还是需要防止重复定义的问题

	sd.Definitions = defs
	sd.Extensions = append(sd.Extensions, exts...)

	// TODO: 这个逻辑其实不应该在这里，这是 gqlparser 的问题，其实可以自写一份 formatter，并且其 formatter 对 extends 的顺序处理的也不到位
	addNewlineAfterDescription(sd)

	// remove dummy directive
	removeDirectives(sd, directiveNode)

	return r, nil
}

func ensureBuiltInNodeFields(typ *ast.Definition) {
	existingFields := lo.SliceToMap(typ.Fields, func(f *ast.FieldDefinition) (string, *ast.FieldDefinition) {
		return f.Name, f
	})

	fieldsToAdd := []*ast.FieldDefinition{
		{Name: "id", Type: ast.NonNullNamedType("ID", nil)},
		{Name: "createdAt", Type: ast.NonNullNamedType("Time", nil)},
		{Name: "updatedAt", Type: ast.NonNullNamedType("Time", nil)},
	}

	for _, field := range fieldsToAdd {
		if _, exists := existingFields[field.Name]; !exists {
			typ.Fields = append(typ.Fields, field)
		}
	}

	sortNodeFields(typ.Fields)
}

var builtInNodeFieldOrder = map[string]int{"id": 1, "createdAt": 2, "updatedAt": 3}

func sortNodeFields(fields []*ast.FieldDefinition) {
	sort.SliceStable(fields, func(i, j int) bool {
		oi, oki := builtInNodeFieldOrder[fields[i].Name]
		oj, okj := builtInNodeFieldOrder[fields[j].Name]
		if oki && okj {
			return oi < oj
		}
		if oki || okj {
			return oki
		}
		return false
	})
}

func connectionMethod(typName, methodName string) *ast.FieldDefinition {
	return &ast.FieldDefinition{
		Name: methodName,
		Type: ast.NonNullNamedType(typName+"Connection", nil),
		Arguments: ast.ArgumentDefinitionList{
			{Name: "after", Type: ast.NamedType("Cursor", nil)},
			{Name: "first", Type: ast.NamedType("Int", nil)},
			{Name: "before", Type: ast.NamedType("Cursor", nil)},
			{Name: "last", Type: ast.NamedType("Int", nil)},
			{Name: "filterBy", Type: ast.NamedType(typName+"Filter", nil)},
			{Name: "orderBy", Type: ast.ListType(ast.NonNullNamedType(typName+"Order", nil), nil)},
		},
	}
}

func ensureFieldConnections(sd *ast.SchemaDocument, typ *ast.Definition) error {
	for idx, field := range typ.Fields {
		if _, exists := reservedFields[field.Name]; exists {
			continue
		}
		// skip non-list type
		if field.Type.Elem == nil {
			continue
		}
		// skip if the type is not an object with node directive
		def := findDefinition(sd, field.Type.Elem.NamedType)
		if def == nil || def.Kind != ast.Object || !directiveExists(def, directiveNode) {
			continue
		}
		if !field.Type.NonNull {
			return errors.Errorf("field %s.%s should be non-null", typ.Name, field.Name)
		}
		if !field.Type.Elem.NonNull {
			return errors.Errorf("elem of field %s.%s should be non-null", typ.Name, field.Name)
		}
		typ.Fields[idx] = connectionMethod(field.Type.Elem.NamedType, field.Name)
	}
	return nil
}

func ensureQuery(sd *ast.SchemaDocument, typ *ast.Definition) (exts []*ast.Definition) {
	methods := parseMethods(sd, "Query")

	var extMethods []*ast.FieldDefinition

	connectionMethodName := lo.CamelCase(inflection.Plural(typ.Name))
	if !methodExists(methods, connectionMethodName) {
		extMethods = append(extMethods, connectionMethod(typ.Name, connectionMethodName))
	}
	if len(extMethods) > 0 {
		exts = append(exts, &ast.Definition{
			Kind:   ast.Object,
			Name:   "Query",
			Fields: extMethods,
		})
	}
	return exts
}

func ensureConnectionTypes(sd *ast.SchemaDocument, typ *ast.Definition) (defs []*ast.Definition) {
	connectionName := typ.Name + "Connection"
	if !definitionExists(sd, connectionName) {
		defs = append(defs, &ast.Definition{
			Kind: ast.Object,
			Name: connectionName,
			Fields: []*ast.FieldDefinition{
				{Name: "nodes", Type: ast.NonNullListType(ast.NonNullNamedType(typ.Name, nil), nil)},
				{Name: "edges", Type: ast.NonNullListType(ast.NonNullNamedType(typ.Name+"Edge", nil), nil)},
				{Name: "pageInfo", Type: ast.NonNullNamedType("PageInfo", nil)},
				{Name: "totalCount", Type: ast.NamedType("Int", nil)},
			},
		})
	}

	edgeName := typ.Name + "Edge"
	if !definitionExists(sd, edgeName) {
		defs = append(defs, &ast.Definition{
			Kind: ast.Object,
			Name: edgeName,
			Fields: []*ast.FieldDefinition{
				{Name: "node", Type: ast.NonNullNamedType(typ.Name, nil)},
				{Name: "cursor", Type: ast.NonNullNamedType("Cursor", nil)},
			},
		})
	}
	return defs
}

func ensureFilter(sd *ast.SchemaDocument, typ *ast.Definition) (defs []*ast.Definition) {
	filterName := typ.Name + "Filter"
	if definitionExists(sd, filterName) {
		return nil
	}
	fields := []*ast.FieldDefinition{
		{Name: "not", Type: ast.NamedType(filterName, nil)},
		{Name: "and", Type: ast.ListType(ast.NonNullNamedType(filterName, nil), nil)},
		{Name: "or", Type: ast.ListType(ast.NonNullNamedType(filterName, nil), nil)},
	}
	filterFields := lo.FilterMap(typ.Fields, func(f *ast.FieldDefinition, _ int) (*ast.FieldDefinition, bool) {
		if _, exists := reservedFields[f.Name]; exists {
			return nil, false
		}
		// skip list type and method type
		if IsListType(f.Type) || IsMethodField(f) {
			return nil, false
		}
		// skip fields that are not scalar or enum
		def := findDefinition(sd, f.Type.NamedType)
		if def != nil && def.Kind != ast.Scalar && def.Kind != ast.Enum {
			if def.Kind == ast.Object && directiveExists(def, directiveNode) {
				return &ast.FieldDefinition{Name: f.Name, Type: ast.NamedType(fmt.Sprintf("%sFilter", def.Name), nil)}, true
			}
			return nil, false
		}
		switch f.Type.Name() {
		case "String":
			return &ast.FieldDefinition{Name: f.Name, Type: ast.NamedType("StringFilter", nil)}, true
		case "Int":
			return &ast.FieldDefinition{Name: f.Name, Type: ast.NamedType("IntFilter", nil)}, true
		case "Float":
			return &ast.FieldDefinition{Name: f.Name, Type: ast.NamedType("FloatFilter", nil)}, true
		case "Boolean":
			return &ast.FieldDefinition{Name: f.Name, Type: ast.NamedType("BooleanFilter", nil)}, true
		case "ID":
			return &ast.FieldDefinition{Name: f.Name, Type: ast.NamedType("IDFilter", nil)}, true
		case "Time":
			return &ast.FieldDefinition{Name: f.Name, Type: ast.NamedType("TimeFilter", nil)}, true
		default:
			if def != nil && def.Kind == ast.Enum {
				return &ast.FieldDefinition{Name: f.Name, Type: ast.NamedType("EnumFilter", nil)}, true
			}
		}
		return nil, false
	})
	fields = append(fields, filterFields...)
	return []*ast.Definition{{
		Kind:   ast.InputObject,
		Name:   filterName,
		Fields: fields,
	}}
}

func ensureOrderTypes(sd *ast.SchemaDocument, typ *ast.Definition) (defs []*ast.Definition) {
	orderName := typ.Name + "Order"
	if !definitionExists(sd, orderName) {
		defs = append(defs, &ast.Definition{
			Kind: ast.InputObject,
			Name: orderName,
			Fields: []*ast.FieldDefinition{
				{Name: "field", Type: ast.NonNullNamedType(typ.Name+"OrderField", nil)},
				{Name: "direction", Type: ast.NonNullNamedType("OrderDirection", nil)},
			},
		})
	}

	orderFieldName := typ.Name + "OrderField"
	if !definitionExists(sd, orderFieldName) {
		enumValues := lo.FilterMap(typ.Fields, func(f *ast.FieldDefinition, _ int) (*ast.EnumValueDefinition, bool) {
			if _, exists := reservedFields[f.Name]; exists {
				return nil, false
			}
			// skip list type and method type
			if IsListType(f.Type) || IsMethodField(f) {
				return nil, false
			}
			// skip fields that are not scalar or enum
			def := findDefinition(sd, f.Type.NamedType)
			if def != nil && def.Kind != ast.Scalar && def.Kind != ast.Enum {
				return nil, false
			}
			return &ast.EnumValueDefinition{Name: strings.ToUpper(lo.SnakeCase(f.Name))}, true
		})
		defs = append(defs, &ast.Definition{
			Kind:       ast.Enum,
			Name:       orderFieldName,
			EnumValues: enumValues,
		})
	}
	return defs
}

func ensureMutation(sd *ast.SchemaDocument, typ *ast.Definition) (exts []*ast.Definition) {
	methods := parseMethods(sd, "Mutation")

	var extMethods []*ast.FieldDefinition

	actions := []string{"create", "update", "delete"}
	for _, action := range actions {
		methodName := action + typ.Name
		if !methodExists(methods, methodName) {
			extMethods = append(extMethods, &ast.FieldDefinition{
				Name: methodName,
				Type: ast.NonNullNamedType(lo.PascalCase(methodName+"Payload"), nil),
				Arguments: ast.ArgumentDefinitionList{
					{Name: "input", Type: ast.NonNullNamedType(lo.PascalCase(methodName+"Input"), nil)},
				},
			})
		}
	}

	if len(extMethods) > 0 {
		exts = append(exts, &ast.Definition{
			Kind:   ast.Object,
			Name:   "Mutation",
			Fields: extMethods,
		})
	}
	return exts
}

func ensureMutationTypes(sd *ast.SchemaDocument, typ *ast.Definition) (defs []*ast.Definition) {
	actions := []string{"create", "update", "delete"}

	for _, action := range actions {
		inputName := lo.PascalCase(action + typ.Name + "Input")
		if !definitionExists(sd, inputName) {
			fields := []*ast.FieldDefinition{
				{Name: "clientMutationId", Type: ast.NamedType("String", nil)},
			}
			if action != "create" {
				fields = append(fields, &ast.FieldDefinition{Name: lo.CamelCase(typ.Name + "Id"), Type: ast.NonNullNamedType("ID", nil)})
			}
			if action != "delete" {
				fields = append(fields, lo.FilterMap(typ.Fields, func(f *ast.FieldDefinition, _ int) (*ast.FieldDefinition, bool) {
					if _, exists := reservedFields[f.Name]; exists {
						return nil, false
					}
					// skip method type
					if IsMethodField(f) {
						return nil, false
					}
					_, exists := builtInNodeFieldOrder[f.Name]
					if exists {
						return nil, false
					}

					typ := clone.Slowly(f.Type).(*ast.Type)
					name := f.Name

					if IsListType(f.Type) {
						// skip list type for now
						// TODO: 这块逻辑还没想好，像 [String!]! 其实应该支持才对，感觉需要通过某种配置指定才合适
						return nil, false

						// // add fieldIds for list type
						// typ = &ast.Type{
						// 	Elem: &ast.Type{
						// 		NamedType: f.Type.Elem.NamedType,
						// 		NonNull:   f.Type.Elem.NonNull,
						// 	},
						// 	NonNull: f.Type.NonNull,
						// }

						// def := findDefinition(sd, f.Type.Elem.NamedType)
						// if def != nil && def.Kind == ast.Object {
						// 	typ.Elem.NamedType = "ID"
						// 	name = inflection.Singular(f.Name) + "Ids"
						// }
					} else {
						def := findDefinition(sd, f.Type.NamedType)
						if def != nil {
							// add fieldId for object type
							if def.Kind == ast.Object {
								// TODO: 这里严格来说或许应该以目标是否有 id 字段为准
								if directiveExists(def, directiveNode) {
									typ.NamedType = "ID"
									name = f.Name + "Id"
								} else {
									// TODO: 这个不完善，满足不了只是嵌套而非关联的情况，这种情况应该需要打标记然后转换成 InputObject 的形式，但是 InputObject 应该怎么定义呢？
									// TODO: 并且这时候可能也要支持 List 类型，并且需要考虑到多层 list 的情况
									return nil, false
								}
							} else if def.Kind != ast.Scalar && def.Kind != ast.Enum {
								// skip fields that are not scalar or enum
								return nil, false
							}
						}
					}

					if action == "update" {
						typ.NonNull = false
					}
					return &ast.FieldDefinition{Name: name, Type: typ}, true
				})...)
			}
			defs = append(defs, &ast.Definition{
				Kind:   ast.InputObject,
				Name:   inputName,
				Fields: fields,
			})
		}

		payloadName := lo.PascalCase(action + typ.Name + "Payload")
		if !definitionExists(sd, payloadName) {
			defs = append(defs, &ast.Definition{
				Kind: ast.Object,
				Name: payloadName,
				Fields: []*ast.FieldDefinition{
					{Name: "clientMutationId", Type: ast.NamedType("String", nil)},
					{Name: lo.CamelCase(typ.Name), Type: ast.NonNullNamedType(typ.Name, nil)},
				},
			})
		}
	}

	return defs
}

func ensureViewerPermission(sd *ast.SchemaDocument, typ *ast.Definition) (defs []*ast.Definition) {
	viewerPermissionName := fmt.Sprintf("%sViewerPermission", typ.Name)

	def := typ.Fields.ForName("viewerPermission")
	if def != nil {
		def.Type = ast.NonNullNamedType(viewerPermissionName, nil)
	} else {
		typ.Fields = append(typ.Fields, &ast.FieldDefinition{
			Name: "viewerPermission",
			Type: ast.NonNullNamedType(viewerPermissionName, nil),
		})
	}

	if !definitionExists(sd, viewerPermissionName) {
		defs = append(defs, &ast.Definition{
			Kind: ast.Object,
			Name: viewerPermissionName,
			Fields: []*ast.FieldDefinition{
				{Name: "canCreate", Type: ast.NonNullNamedType("Boolean", nil)},
				{Name: "canUpdate", Type: ast.NonNullNamedType("Boolean", nil)},
				{Name: "canDelete", Type: ast.NonNullNamedType("Boolean", nil)},
			},
		})
	}
	return defs
}

func directiveExists(def *ast.Definition, directiveName string) bool {
	return def.Directives.ForName(directiveName) != nil
}

func findDefinition(sd *ast.SchemaDocument, name string) *ast.Definition {
	index := slices.IndexFunc(sd.Definitions, func(def *ast.Definition) bool {
		return def.Name == name
	})
	if index >= 0 {
		return sd.Definitions[index]
	}
	return nil
}

func definitionExists(sd *ast.SchemaDocument, name string) bool {
	return findDefinition(sd, name) != nil
}

func methodExists(methods []*ast.FieldDefinition, name string) bool {
	return slices.ContainsFunc(methods, func(def *ast.FieldDefinition) bool {
		return def.Name == name
	})
}

func parseMethods(sd *ast.SchemaDocument, name string) []*ast.FieldDefinition {
	defs := append(
		lo.Filter(sd.Definitions, func(def *ast.Definition, _ int) bool {
			return def.Name == name
		}),
		lo.Filter(sd.Extensions, func(def *ast.Definition, _ int) bool {
			return def.Name == name
		})...,
	)
	return lo.FlatMap(defs, func(def *ast.Definition, _ int) []*ast.FieldDefinition {
		return def.Fields
	})
}

func addNewlineAfterDescription(sd *ast.SchemaDocument) {
	for _, def := range append(sd.Definitions, sd.Extensions...) {
		if def.AfterDescriptionComment == nil {
			def.AfterDescriptionComment = &ast.CommentGroup{}
		}
		def.AfterDescriptionComment.List = append(def.AfterDescriptionComment.List, &ast.Comment{Value: "\n"})
	}
}

func removeDirectives(sd *ast.SchemaDocument, directiveNames ...string) {
	sd.Directives = lo.Filter(sd.Directives, func(d *ast.DirectiveDefinition, _ int) bool {
		return !slices.Contains(directiveNames, d.Name)
	})
	for _, def := range append(sd.Definitions, sd.Extensions...) {
		def.Directives = lo.Filter(def.Directives, func(d *ast.Directive, _ int) bool {
			return !slices.Contains(directiveNames, d.Name)
		})
		for _, def := range def.Fields {
			def.Directives = lo.Filter(def.Directives, func(d *ast.Directive, _ int) bool {
				return !slices.Contains(directiveNames, d.Name)
			})
		}
	}
}
