package main

//go:generate go run . --output-dir=../../

import (
	"context"
	"log"
	"os"

	"github.com/molon/genx"
	"github.com/molon/genx/extension/gosurgery"
	"github.com/molon/genx/extension/gqlgenext"
	"github.com/molon/genx/extension/relayext"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

var outputDir = pflag.StringP("output-dir", "o", ".", "output directory")

func main() {
	pflag.Parse()
	if err := Generate(context.Background()); err != nil {
		log.Fatalf("failed to generate: %+v", err)
	}
}

func Generate(ctx context.Context) error {
	if outputDir == nil || *outputDir == "" {
		return errors.New("output dir is required")
	}
	if err := os.Chdir(*outputDir); err != nil {
		return errors.Wrap(err, "failed to change working directory")
	}
	if err := genx.Generate(ctx, &genx.Config{
		OutputDir:           ".",
		PrototypeRelPattern: "prototype.graphql",
		GoModule:            "github.com/molon/genx/starter/boilerplate",
		Extensions: []genx.Extension{
			relayext.New(),
			gosurgery.New(),
			gqlgenext.New(),
		},
	}); err != nil {
		return errors.Wrap(err, "failed to generate")
	}

	return nil
}
