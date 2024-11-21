package gqlgenext

import (
	"context"

	"github.com/99designs/gqlgen/plugin"

	"github.com/molon/genx"
	"github.com/pkg/errors"

	gqlapi "github.com/99designs/gqlgen/api"
	gqlconfig "github.com/99designs/gqlgen/codegen/config"
)

type WithGQLPlugins interface {
	GQLPlugins() []plugin.Plugin
}

var _ genx.Extension = (*Extension)(nil)

type Extension struct {
	genx.DefaultExtension
	generatedFiles []*genx.File
}

func New() *Extension {
	return &Extension{}
}

func (e *Extension) Name() string {
	return "gqlgenext"
}

func (e *Extension) AfterGenerate(ctx context.Context, r *genx.Runtime) error {
	gqlconf, err := gqlconfig.LoadConfig("gqlgen.yml")
	if err != nil {
		return errors.Wrap(err, "failed to load gqlgen config")
	}

	var options []gqlapi.Option
	for _, ext := range r.Config.Extensions {
		if ext == e {
			continue
		}
		if withGQLPlugins, ok := ext.(WithGQLPlugins); ok {
			for _, p := range withGQLPlugins.GQLPlugins() {
				options = append(options, gqlapi.AddPlugin(p))
			}
		}
	}

	if err := gqlapi.Generate(gqlconf, options...); err != nil {
		return errors.Wrap(err, "failed to generate gqlgen")
	}
	return nil
}
