package genx

import (
	"context"

	"github.com/vektah/gqlparser/v2/ast"
)

type Config struct {
	OutputDir           string
	PrototypeRelPattern string
	GoModule            string
	Extensions          []Extension
}

type Runtime struct {
	*Config
	Schema  *ast.Schema
	Results map[string]*Result
}

type Result struct {
	Files    []*File
	Metadata any
}

type Extension interface {
	Name() string
	BeforeGenerate(ctx context.Context, r *Runtime) error
	Generate(ctx context.Context, r *Runtime) (*Result, error)
	AfterGenerate(ctx context.Context, r *Runtime) error
}

var _ Extension = &DefaultExtension{}

type DefaultExtension struct{}

func (e *DefaultExtension) BeforeGenerate(ctx context.Context, r *Runtime) error {
	return nil
}

func (e *DefaultExtension) Name() string {
	panic("implement me")
}

func (e *DefaultExtension) Generate(ctx context.Context, r *Runtime) (*Result, error) {
	return &Result{}, nil
}

func (e *DefaultExtension) AfterGenerate(ctx context.Context, r *Runtime) error {
	return nil
}
