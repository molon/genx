package relayext

import (
	"bytes"
	"context"
	"path/filepath"
	"text/template"

	_ "embed"

	"github.com/molon/genx"
	"github.com/pkg/errors"
)

//go:embed embed/models.tmpl
var modelsTmpl string

func (e *Extension) generateModels(_ context.Context, data *Data) ([]*genx.File, error) {
	tmpl, err := template.New("models").Funcs(Funcs).Parse(modelsTmpl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse template")
	}

	var buf bytes.Buffer
	buf.WriteString("// " + header)
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, errors.Wrapf(err, "failed to execute template")
	}

	return []*genx.File{
		{
			RelPath: filepath.Join("server", "model", "models.genx.go"),
			Content: buf.String(),
		},
	}, nil
}
