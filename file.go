package genx

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
)

type File struct {
	RelPath string
	Content string
}

func (f *File) Format(ctx context.Context) error {
	content, err := formatText(ctx, filepath.Ext(f.RelPath), f.Content)
	if err != nil {
		return errors.Wrapf(err, "failed to format %s", f.RelPath)
	}
	f.Content = content
	return nil
}

func (f *File) ApplyReplacements(ctx context.Context, replacements Replacements) error {
	text, err := replacements.Apply(f.Content)
	if err != nil {
		return errors.Wrapf(err, "failed to apply replacements to %s", f.RelPath)
	}
	content, err := formatText(ctx, filepath.Ext(f.RelPath), text)
	if err != nil {
		return errors.Wrapf(err, "failed to format %s after applying replacements", f.RelPath)
	}
	f.Content = content
	return nil
}
