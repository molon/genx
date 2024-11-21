package relayext

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"text/template"

	_ "embed"

	"github.com/molon/genx"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

func (e *Extension) generateResolvers(ctx context.Context, data *Data) ([]*genx.File, error) {
	var generatedFiles []*genx.File

	rootResolverFiles, err := e.generateRootResolver(ctx, data)
	if err != nil {
		return nil, err
	}
	generatedFiles = append(generatedFiles, rootResolverFiles...)

	for _, node := range data.Nodes {
		nodeResolverFiles, err := e.generateNodeResolver(ctx, data, node)
		if err != nil {
			return nil, err
		}
		generatedFiles = append(generatedFiles, nodeResolverFiles...)
	}
	return generatedFiles, nil
}

//go:embed embed/root_resolver.tmpl
var rootResolverTmpl string

func (e *Extension) generateRootResolver(_ context.Context, data *Data) ([]*genx.File, error) {
	tmpl, err := template.New("resolver.tmpl").Funcs(Funcs).Parse(rootResolverTmpl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse resolver template")
	}

	var buf bytes.Buffer
	buf.WriteString(header)
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, errors.Wrapf(err, "failed to execute resolver template")
	}

	return []*genx.File{
		{
			RelPath: filepath.Join("server", "resolver", "resolver.genx.go"),
			Content: buf.String(),
		},
	}, nil
}

//go:embed embed/node_resolver.tmpl
var nodeResolverTmpl string

func (e *Extension) generateNodeResolver(_ context.Context, data *Data, node *Node) ([]*genx.File, error) {
	tmpl, err := template.New("node_resolver.tmpl").Funcs(Funcs).Parse(nodeResolverTmpl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse node resolver template")
	}

	var buf bytes.Buffer
	buf.WriteString(header)
	if err := tmpl.Execute(&buf, struct {
		*Data
		*Node
	}{
		Data: data,
		Node: node,
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to execute node resolver template")
	}

	return []*genx.File{
		{
			RelPath: filepath.Join("server", "resolver", fmt.Sprintf("%s_resolver.genx.go", lo.SnakeCase(node.Name))),
			Content: buf.String(),
		},
	}, nil
}
