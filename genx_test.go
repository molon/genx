package genx_test

import (
	"context"
	"testing"

	"github.com/molon/genx"
	"github.com/molon/genx/extension/relayext"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	err := genx.Generate(context.Background(), &genx.Config{
		OutputDir:           "./__testdata",
		PrototypeRelPattern: "prototype.graphql",
		GoModule:            "github.com/molon/genx/__testdata",
	}, genx.Extensions(
		relayext.New(),
	))
	require.NoError(t, err)
	// TODO:
}
