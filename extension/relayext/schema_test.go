package relayext

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/molon/genx/pkg/gqlx"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func TestEnhanceSchema(t *testing.T) {
	sd, err := parser.ParseSchemas(
		&ast.Source{
			Name: "prototype.graphql",
			Input: `
type Company @node {
  name: String!
  description: String
  manager: User
  employees: [User!]!
}

type User @node {
  name: String!
  description: String
  age: Int!
  company: Company!
  viewerPermission: CompanyViewerPermission!
}

				`,
		},
	)
	require.NoError(t, err)

	r, err := enhanceSchema(context.Background(), sd)
	require.NoError(t, err)

	result := gqlx.FormatDocument(r.Document)
	os.WriteFile("./__testdata/schema.genx.graphql", []byte(result), os.ModePerm)

	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.genx.graphql", Input: result})
	require.NoError(t, err)
	for _, t := range schema.Types {
		fmt.Printf("Generated type: %s\n", t.Name)
	}
	// TODO：校验生成的 schema 是否符合预期
}
