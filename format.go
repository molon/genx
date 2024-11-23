package genx

import (
	"context"

	"github.com/pkg/errors"
	"golang.org/x/tools/imports"
	"mvdan.cc/gofumpt/format"
)

func FormatText(_ context.Context, ext, text string) (string, error) {
	switch ext {
	case ".go":
		formatted, err := FormatGo(text)
		if err != nil {
			return "", errors.Wrapf(err, "failed to format %s", ext)
		}
		return formatted, nil
	}
	return text, nil
}

func FormatGo(source string) (string, error) {
	formatted, err := imports.Process("dummy.go", []byte(source), &imports.Options{
		AllErrors:  false,
		Comments:   true,
		FormatOnly: false,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to format Go source with goimports")
	}

	formatted, err = format.Source(formatted, format.Options{
		LangVersion: "go1.23.1",
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to format Go source with gofumpt")
	}
	return string(formatted), nil
}
