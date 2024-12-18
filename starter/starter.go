package starter

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	_ "embed"
)

// zip boilerplate directory without files ignored by .gitignore
//go:generate sh -c "rm -f boilerplate.zip && cd boilerplate && git ls-files --cached --others --exclude-standard -z | xargs -0 zip -q -X -o ../boilerplate.zip"

//go:embed boilerplate.zip
var boilerplateZip []byte

var boilerplateGoModule = []byte("github.com/molon/genx/starter/boilerplate")

func Extract(ctx context.Context, conf *Config) error {
	if conf.GoModule == "" {
		return errors.New("goModule is required")
	}

	targetDir := conf.TargetDir
	if targetDir == "" {
		dir, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get working directory")
		}
		targetDir = dir
	}

	if err := checkTargetDir(targetDir); err != nil {
		return err
	}

	var reader io.ReaderAt
	var size int64
	if conf.BoilerplateZipFile != "" {
		file, err := os.Open(conf.BoilerplateZipFile)
		if err != nil {
			return errors.Wrapf(err, "failed to open boilerplate zip file: %s", conf.BoilerplateZipFile)
		}
		defer file.Close()

		fileInfo, err := file.Stat()
		if err != nil {
			return errors.Wrap(err, "failed to get file info")
		}

		reader = file
		size = fileInfo.Size()
	} else {
		reader = bytes.NewReader(boilerplateZip)
		size = int64(len(boilerplateZip))
	}

	goModFileModified := false
	if err := extractZip(ctx, reader, size, targetDir, func(ctx context.Context, path string, content []byte) ([]byte, error) {
		content = bytes.ReplaceAll(content, boilerplateGoModule, []byte(conf.GoModule))
		if strings.HasSuffix(path, "/go.mod") || path == "go.mod" {
			content = bytes.ReplaceAll(content, []byte("replace github.com/molon/genx => ../../"), []byte{})
			goModFileModified = true
		}
		return content, nil
	}); err != nil {
		return err
	}

	if goModFileModified {
		if err := updateGoDependency(targetDir, "github.com/molon/genx", "latest"); err != nil {
			return err
		}
	}
	return nil
}

func extractZip(ctx context.Context, reader io.ReaderAt, size int64, targetDir string, modifier func(ctx context.Context, path string, content []byte) ([]byte, error)) error {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return errors.Wrap(err, "failed to create zip reader")
	}

	for _, file := range zipReader.File {
		path := filepath.Join(targetDir, file.Name)

		// Ensure the path is secure
		relPath, err := filepath.Rel(targetDir, path)
		if err != nil || strings.HasPrefix(relPath, "..") {
			return errors.Errorf("illegal file path: %s", path)
		}

		// Ensure the parent directory exists
		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			return errors.Wrapf(err, "failed to create directory for file: %s", path)
		}

		// Handle directories (if explicitly marked)
		if file.FileInfo().IsDir() {
			// Create the directory (even if MkdirAll above ensures it exists)
			if err := os.MkdirAll(path, os.ModePerm); err != nil {
				return errors.Wrap(err, "failed to create directory")
			}
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return errors.Wrap(err, "failed to open file in zip")
		}

		var buf bytes.Buffer
		if _, err := io.Copy(&buf, fileReader); err != nil {
			fileReader.Close()
			return errors.Wrap(err, "failed to read file content")
		}
		fileReader.Close()

		content := buf.Bytes()
		if modifier != nil {
			modifiedContent, err := modifier(ctx, path, content)
			if err != nil {
				return err
			}
			content = modifiedContent
		}

		// Write the file with original permissions
		if err := os.WriteFile(path, content, file.Mode()); err != nil {
			return errors.Wrapf(err, "failed to write file: %s", path)
		}
	}

	return nil
}

func checkTargetDir(targetDir string) error {
	info, err := os.Stat(targetDir)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to check target directory")
	}
	if err == nil {
		if !info.IsDir() {
			return errors.Errorf("target %s is not a directory", targetDir)
		}

		dir, err := os.Open(targetDir)
		if err != nil {
			return errors.Wrap(err, "failed to open target directory")
		}
		defer dir.Close()

		_, err = dir.Readdirnames(1)
		if err == nil {
			return errors.Errorf("target %s is not empty", targetDir)
		} else if err != io.EOF {
			return err
		}
	}
	return nil
}

func updateGoDependency(targetDir, module, version string) error {
	dep := fmt.Sprintf("%s@%s", module, version)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("go get %s && go mod tidy", dep))
	cmd.Dir = targetDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to run 'go get %s && go mod tidy'", dep)
	}
	return nil
}
