package relayext

import (
	"go/token"
	"go/types"

	"github.com/vektah/gqlparser/v2/ast"
)

func IsListType(t *ast.Type) bool {
	return t.Elem != nil
}

func IsMethodField(f *ast.FieldDefinition) bool {
	return f.Arguments != nil && len(f.Arguments) > 0
}

func IsGORMModel(def *ast.Definition) bool {
	id := def.Fields.ForName("id")
	createdAt := def.Fields.ForName("createdAt")
	updatedAt := def.Fields.ForName("updatedAt")
	return id != nil && id.Type.Name() == "ID" &&
		createdAt != nil && createdAt.Type.Name() == "Time" &&
		updatedAt != nil && updatedAt.Type.Name() == "Time"
}

func NewEnumType(name string) types.Type {
	return types.NewNamed(
		types.NewTypeName(token.NoPos, nil, name, nil),
		types.Typ[types.String],
		nil,
	)
}

func NewTimeType() types.Type {
	return types.NewNamed(
		types.NewTypeName(token.NoPos, types.NewPackage("time", "time"), "Time", nil),
		nil,
		nil,
	)
}

func TypeString(typ types.Type) string {
	return types.TypeString(typ, func(pkg *types.Package) string {
		if pkg != nil {
			return pkg.Name()
		}
		return ""
	})
}

func IsPointerType(typ types.Type) bool {
	_, ok := typ.(*types.Pointer)
	return ok
}
