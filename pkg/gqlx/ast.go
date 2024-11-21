package gqlx

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
)

func FormatDocument(doc *ast.SchemaDocument) string {
	sb := &strings.Builder{}
	formatter.NewFormatter(sb, formatter.WithIndent("  "), formatter.WithComments()).FormatSchemaDocument(doc)
	return sb.String()
}

func LoadSources(pattern string) ([]*ast.Source, error) {
	matchedFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to glob files matching pattern %s", pattern)
	}
	if len(matchedFiles) == 0 {
		return nil, errors.Errorf("no files found matching pattern %s", pattern)
	}
	var sources []*ast.Source
	for _, file := range matchedFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file %s", file)
		}
		sources = append(sources, &ast.Source{Name: filepath.Base(file), Input: string(content)})
	}
	return sources, nil
}
