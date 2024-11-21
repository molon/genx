package starter

import (
	"github.com/molon/genx/pkg/configx"
	"github.com/spf13/pflag"
)

type Config struct {
	TargetDir string `mapstructure:"targetDir"`
	GoModule  string `mapstructure:"goModule"`
	ZipFile   string `mapstructure:"zipFile"`
}

func InitializeConfig(flagSet *pflag.FlagSet, envPrefix string) (configx.Loader[*Config], error) {
	return configx.Initialize(flagSet, envPrefix, &Config{
		TargetDir: ".",
		GoModule:  "",
		ZipFile:   "",
	})
}
