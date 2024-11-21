package genx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/molon/genx/pkg/gqlx"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2"
)

func Generate(ctx context.Context, config *Config, options ...Option) error {
	if err := applyOptions(config, options...); err != nil {
		return err
	}
	if err := validateConfig(config); err != nil {
		return err
	}
	runtime := &Runtime{
		Config:  config,
		Schema:  nil,
		Results: make(map[string]*Result),
	}
	if err := beforeGenerate(ctx, runtime); err != nil {
		return err
	}
	if runtime.Schema == nil {
		if err := loadSchema(config, runtime); err != nil {
			return err
		}
	}
	if err := generateFiles(ctx, runtime); err != nil {
		return err
	}
	if err := checkDuplicateFiles(runtime); err != nil {
		return err
	}
	if err := formatFiles(ctx, runtime.Results); err != nil {
		return err
	}
	if err := writeFiles(config.OutputDir, runtime.Results); err != nil {
		return err
	}
	if err := afterGenerate(ctx, runtime); err != nil {
		return err
	}
	return nil
}

func applyOptions(config *Config, options ...Option) error {
	for _, opt := range options {
		if err := opt(config); err != nil {
			return err
		}
	}
	return nil
}

func validateConfig(config *Config) error {
	if config.OutputDir == "" {
		return errors.New("output dir is required")
	}
	if config.PrototypeRelPattern == "" {
		return errors.New("prototype rel pattern is required")
	}
	if len(config.Extensions) == 0 {
		return errors.New("no extensions")
	}
	duplicatedExtensions := lo.Map(lo.FindDuplicatesBy(config.Extensions, func(ext Extension) string {
		return ext.Name()
	}), func(ext Extension, _ int) string { return ext.Name() })
	if len(duplicatedExtensions) > 0 {
		return errors.Errorf("duplicated extensions: %s", duplicatedExtensions)
	}
	return nil
}

func beforeGenerate(ctx context.Context, runtime *Runtime) error {
	for _, ext := range runtime.Config.Extensions {
		if err := ext.BeforeGenerate(ctx, runtime); err != nil {
			return err
		}
	}
	return nil
}

func afterGenerate(ctx context.Context, runtime *Runtime) error {
	for _, ext := range runtime.Config.Extensions {
		if err := ext.AfterGenerate(ctx, runtime); err != nil {
			return err
		}
	}
	return nil
}

func loadSchema(config *Config, runtime *Runtime) error {
	prototypePattern := filepath.Join(config.OutputDir, config.PrototypeRelPattern)
	sources, err := gqlx.LoadSources(prototypePattern)
	if err != nil {
		return err
	}
	schema, err := gqlparser.LoadSchema(sources...)
	if err != nil {
		return errors.Wrap(err, "failed to validate schema")
	}
	runtime.Schema = schema
	return nil
}

func generateFiles(ctx context.Context, runtime *Runtime) error {
	for _, ext := range runtime.Config.Extensions {
		result, err := ext.Generate(ctx, runtime)
		if err != nil {
			return err
		}
		runtime.Results[ext.Name()] = result
	}
	return nil
}

func checkDuplicateFiles(runtime *Runtime) error {
	fileToExtensions := make(map[string][]string)
	for extName, result := range runtime.Results {
		for _, file := range result.Files {
			fileToExtensions[file.RelPath] = append(fileToExtensions[file.RelPath], extName)
		}
	}
	var duplicatedFiles []string
	for filePath, extensions := range fileToExtensions {
		if len(extensions) > 1 {
			duplicatedFiles = append(duplicatedFiles, fmt.Sprintf("file: %s, extensions: %v", filePath, extensions))
		}
	}
	if len(duplicatedFiles) > 0 {
		return errors.Errorf("duplicate files detected:\n%s", strings.Join(duplicatedFiles, "\n"))
	}
	return nil
}

func formatFiles(ctx context.Context, results map[string]*Result) error {
	for _, result := range results {
		for _, file := range result.Files {
			if err := file.Format(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeFiles(outputDir string, results map[string]*Result) error {
	for _, result := range results {
		for _, file := range result.Files {
			path := filepath.Join(outputDir, file.RelPath)
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return errors.Wrapf(err, "failed to create directory %s", dir)
			}
			if err := os.WriteFile(path, []byte(file.Content), os.ModePerm); err != nil {
				return errors.Wrapf(err, "failed to write file %s", path)
			}
		}
	}
	return nil
}
