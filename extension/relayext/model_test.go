package relayext

import (
	"context"
	"os"
	"testing"

	"github.com/molon/genx"
	"github.com/molon/genx/pkg/jsonx"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestGenerateModels(t *testing.T) {
	e := New()

	input, err := os.ReadFile("./testdata/schema.genx.graphql")
	require.NoError(t, err)

	s, err := gqlparser.LoadSchema(&ast.Source{
		Name:  "schema.genx.graphql",
		Input: string(input),
	})
	require.NoError(t, err)

	r := &genx.Runtime{
		Config: &genx.Config{GoModule: "github.com/molon/genx/__testdata"},
		Schema: s,
	}

	generatedFiles, err := e.generateModels(context.Background(), NewData(r, nil))
	require.NoError(t, err)

	// TODO：校验生成的结果是否符合预期
	t.Logf("generatedFiles: %s", jsonx.MustMarshalToString(generatedFiles))
}
