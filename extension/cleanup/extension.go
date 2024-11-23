package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

	"github.com/molon/genx"
	"github.com/pkg/errors"
)

var _ genx.Extension = (*Extension)(nil)

type Extension struct {
	genx.DefaultExtension
}

func New() *Extension {
	return &Extension{}
}

func (e *Extension) Name() string {
	return "cleanup"
}

var reGeneratedFileSuffix = regexp.MustCompile(`\.genx\.\w+$`)

func (e *Extension) AfterGenerate(ctx context.Context, r *genx.Runtime) error {
	dirToFiles := make(map[string]map[string]bool)
	for _, result := range r.Results {
		for _, f := range result.Files {
			dir := filepath.Dir(filepath.Join(r.OutputDir, f.RelPath))
			m, ok := dirToFiles[dir]
			if !ok {
				m = make(map[string]bool)
				dirToFiles[dir] = m
			}
			m[filepath.Base(f.RelPath)] = true
		}
	}

	for dir, files := range dirToFiles {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			if !reGeneratedFileSuffix.MatchString(base) {
				return nil
			}
			if _, ok := files[base]; ok {
				return nil
			}
			if err := os.Remove(path); err != nil {
				return errors.Wrapf(err, "failed to remove %s", path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
