package relayext

import (
	"fmt"
	"go/token"
	"go/types"
	"slices"
	"sort"
	"strings"

	_ "embed"

	"github.com/molon/genx"
	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2/ast"
)

type Field interface {
	GoName() string
	GoType() types.Type
	GoTag() string
}

type ASTField struct {
	*ast.FieldDefinition
	Node *Node
}

func (f *ASTField) GoName() string {
	name := lo.PascalCase(f.FieldDefinition.Name)
	if strings.HasSuffix(name, "Id") {
		name = strings.TrimSuffix(name, "Id") + "ID"
	}
	if f.isNodeType() {
		return name + "ID"
	}
	return name
}

func (f *ASTField) GoType() types.Type {
	var goType types.Type

	if f.isNodeType() {
		goType = types.Typ[types.String]
	} else {
		switch f.FieldDefinition.Type.Name() {
		case "Int":
			goType = types.Typ[types.Int]
		case "Float":
			goType = types.Typ[types.Float64]
		case "String":
			goType = types.Typ[types.String]
		case "Boolean":
			goType = types.Typ[types.Bool]
		case "ID":
			goType = types.Typ[types.String]
		case "Time":
			goType = NewTimeType()
		// TODO: add a duration type ??
		default:
			if def, ok := f.Node.Schema.Types[f.Type.Name()]; ok {
				if def.Kind == ast.Enum {
					goType = NewEnumType(f.Type.Name())
				}
			}
			if goType == nil {
				panic(fmt.Sprintf("unsupported type %s", f.Type.Name()))
			}
		}
	}

	if !f.Type.NonNull {
		goType = types.NewPointer(goType)
	}

	return goType
}

func (f *ASTField) GoTag() string {
	if f.Type.NonNull {
		return fmt.Sprintf(`gorm:"not null" json:"%s"`, lo.CamelCase(f.GoName()))
	}
	return fmt.Sprintf(`json:"%s,omitempty"`, lo.CamelCase(f.GoName()))
}

func (f *ASTField) isNodeType() bool {
	typeName := f.Type.Name()
	if nodeType, ok := f.Node.Schema.Types[typeName]; ok {
		return f.Node.isNodeType(nodeType)
	}
	return false
}

type GoField struct {
	Name string
	Type types.Type
	Tag  string
}

func (f *GoField) GoName() string {
	return f.Name
}

func (f *GoField) GoType() types.Type {
	return f.Type
}

func (f *GoField) GoTag() string {
	return f.Tag
}

type Input struct {
	*ast.Definition
	Node *Node
}

func (i *Input) Fields() []Field {
	return lo.FilterMap(i.Definition.Fields, func(f *ast.FieldDefinition, _ int) (Field, bool) {
		if f.Name == "clientMutationId" {
			return nil, false
		}
		if f.Name == lo.CamelCase(i.Node.Name)+"Id" {
			return nil, false
		}
		return &ASTField{f, i.Node}, true
	})
}

type ViewerPermission struct {
	*ast.Definition
	Node *Node
}

var reservedPermissionFields = map[string]struct{}{
	"canCreate": {},
	"canUpdate": {},
	"canDelete": {},
}

func (vp *ViewerPermission) Fields() []Field {
	return lo.FilterMap(vp.Definition.Fields, func(f *ast.FieldDefinition, _ int) (Field, bool) {
		switch f.Name {
		case "canCreate":
			if vp.Node.CreateInput() == nil {
				return nil, false
			}
			return &ASTField{f, vp.Node}, true
		case "canUpdate":
			if vp.Node.UpdateInput() == nil {
				return nil, false
			}
			return &ASTField{f, vp.Node}, true
		case "canDelete":
			if vp.Node.DeleteInput() == nil {
				return nil, false
			}
			return &ASTField{f, vp.Node}, true
		}
		// TODO: field permission
		// TODO: can read ???
		return nil, false
	})
}

type Node struct {
	*ast.Definition
	Schema     *ast.Schema
	isNodeType func(typ *ast.Definition) bool
}

func (n *Node) ViewerPermission() *ViewerPermission {
	def := n.Schema.Types[fmt.Sprintf("%sViewerPermission", n.Name)]
	if def == nil || def.Kind != ast.Object {
		return nil
	}
	vp := &ViewerPermission{def, n}
	if len(vp.Fields()) == 0 {
		return nil
	}
	return vp
}

func (n *Node) CreateInput() *Input {
	def := n.Schema.Types[fmt.Sprintf("Create%sInput", n.Name)]
	if def == nil || def.Kind != ast.InputObject {
		return nil
	}
	return &Input{def, n}
}

func (n *Node) UpdateInput() *Input {
	def := n.Schema.Types[fmt.Sprintf("Update%sInput", n.Name)]
	if def == nil || def.Kind != ast.InputObject {
		return nil
	}
	return &Input{def, n}
}

func (n *Node) DeleteInput() *Input {
	def := n.Schema.Types[fmt.Sprintf("Delete%sInput", n.Name)]
	if def == nil || def.Kind != ast.InputObject {
		return nil
	}
	return &Input{def, n}
}

func (n *Node) OneToOne() []*ast.FieldDefinition {
	return lo.FilterMap(n.Definition.Fields, func(f *ast.FieldDefinition, _ int) (*ast.FieldDefinition, bool) {
		typ, ok := n.Schema.Types[f.Type.Name()]
		if !ok {
			return nil, false
		}
		if !n.isNodeType(typ) {
			return nil, false
		}
		return f, true
	})
}

func (n *Node) OneToMany() []*ast.FieldDefinition {
	return lo.FilterMap(n.Definition.Fields, func(f *ast.FieldDefinition, _ int) (*ast.FieldDefinition, bool) {
		typName := f.Type.Name()
		if !strings.HasSuffix(typName, "Connection") {
			return nil, false
		}
		targetTypeName := strings.TrimSuffix(typName, "Connection")
		targetType, ok := n.Schema.Types[targetTypeName]
		if !ok {
			return nil, false
		}
		if !n.isNodeType(targetType) {
			return nil, false
		}
		return f, true
	})
}

func (n *Node) Field(goName string) Field {
	f, _ := lo.Find(n.Fields(), func(f Field) bool {
		return f.GoName() == goName
	})
	return f
}

func (n *Node) Fields() []Field {
	fields := lo.FilterMap(n.Definition.Fields, func(f *ast.FieldDefinition, _ int) (Field, bool) {
		if _, exists := reservedFields[f.Name]; exists {
			return nil, false
		}
		if IsMethodField(f) {
			return nil, false
		}
		return &ASTField{f, n}, true
	})
	if !IsGORMModel(n.Definition) {
		return fields
	}

	newDeletedAtField := func() Field {
		return &GoField{
			Name: "DeletedAt",
			Type: types.NewNamed(
				types.NewTypeName(token.NoPos, types.NewPackage("gorm.io/gorm", "gorm"), "DeletedAt", nil),
				nil,
				nil,
			),
			Tag: `gorm:"index" json:"deletedAt"`,
		}
	}

	lastIndex := -1
	deletedAtExists := false
	for i, f := range fields {
		switch f.GoName() {
		case "ID":
			lastIndex = i
			fields[i] = &GoField{
				Name: "ID",
				Type: types.Typ[types.String],
				Tag:  `gorm:"primaryKey" json:"id"`,
			}
		case "CreatedAt":
			lastIndex = i
			fields[i] = &GoField{
				Name: "CreatedAt",
				Type: NewTimeType(),
				Tag:  `gorm:"index;not null" json:"createdAt"`,
			}
		case "UpdatedAt":
			lastIndex = i
			fields[i] = &GoField{
				Name: "UpdatedAt",
				Type: NewTimeType(),
				Tag:  `gorm:"index;not null" json:"updatedAt"`,
			}
		case "DeletedAt":
			deletedAtExists = true
			fields[i] = newDeletedAtField()
		}
	}
	if !deletedAtExists && lastIndex >= 0 {
		fields = slices.Insert(fields, lastIndex+1, newDeletedAtField())
	}
	return fields
}

type Data struct {
	Nodes    []*Node
	GoModule string
}

func (d *Data) GetNode(name string) *Node {
	node, _ := lo.Find(d.Nodes, func(n *Node) bool {
		return n.Name == name
	})
	return node
}

func NewData(r *genx.Runtime, isNodeType func(def *ast.Definition) bool) *Data {
	if isNodeType == nil {
		isNodeType = func(def *ast.Definition) bool {
			return def.Kind == ast.Object && def.Directives.ForName(directiveNode) != nil
		}
	}
	nodeTypes := lo.PickBy(r.Schema.Types, func(key string, def *ast.Definition) bool {
		return isNodeType(def)
	})
	nodes := lo.MapToSlice(nodeTypes, func(key string, def *ast.Definition) *Node {
		return &Node{Definition: def, Schema: r.Schema, isNodeType: isNodeType}
	})
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
		// if nodes[i].Position.Src.Name < nodes[j].Position.Src.Name {
		// 	return true
		// }
		// if nodes[i].Position.Src.Name > nodes[j].Position.Src.Name {
		// 	return false
		// }
		// return nodes[i].Position.Line < nodes[j].Position.Line
	})
	return &Data{
		Nodes:    nodes,
		GoModule: r.GoModule,
	}
}
