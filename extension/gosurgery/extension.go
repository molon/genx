package gosurgery

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/molon/genx"
	"github.com/pkg/errors"
)

var _ genx.Extension = (*Extension)(nil)

type Extension struct {
	genx.DefaultExtension
	generatedFiles []*genx.File
}

func New() *Extension {
	return &Extension{}
}

func (e *Extension) Name() string {
	return "gosurgery"
}

func (e *Extension) Generate(ctx context.Context, r *genx.Runtime) (*genx.Result, error) {
	genDirToFile := make(map[string][]*genx.File)
	for _, result := range r.Results {
		for _, v := range result.Files {
			base := filepath.Base(v.RelPath)
			if !strings.HasSuffix(base, ".genx.go") {
				continue
			}
			dir := filepath.Dir(v.RelPath)
			genDirToFile[dir] = append(genDirToFile[dir], v)
		}
	}

	userDirToFile := make(map[string][]*genx.File)
	for dir := range genDirToFile {
		dirRealPath := filepath.Join(r.OutputDir, dir)
		userFiles, err := collectFiles(dirRealPath, func(info os.FileInfo) bool {
			return strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), ".genx.go")
		})
		if err != nil {
			return nil, err
		}
		userDirToFile[dir] = userFiles
	}

	for dir, genFiles := range genDirToFile {
		userFiles := userDirToFile[dir]
		if err := Surgery(ctx, genFiles, userFiles); err != nil {
			return nil, errors.Wrapf(err, "surgery in dir %s", dir)
		}
	}

	return &genx.Result{}, nil
}

var rePackageName = regexp.MustCompile(`package\s*(\w+)\s+`)

// TODO: 上面的逻辑还需要考虑到一个文件夹下，多个 package name 的情况
func getPackageName(code string) string {
	matches := rePackageName.FindStringSubmatch(code)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func collectFiles(dir string, filter func(os.FileInfo) bool) ([]*genx.File, error) {
	var files []*genx.File
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || (filter != nil && !filter(info)) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "read file %s", path)
		}
		files = append(files, &genx.File{
			RelPath: path,
			Content: string(content),
		})
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "collect files in dir %s", dir)
	}
	return files, nil
}
