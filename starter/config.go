package starter

import (
	"github.com/molon/genx/pkg/configx"
	"github.com/spf13/pflag"
)

type Config struct {
	TargetDir          string `mapstructure:"targetDir" usage:"target directory to extract boilerplate"`
	GoModule           string `mapstructure:"goModule" usage:"go module path to replace in boilerplate"`
	BoilerplateZipFile string `mapstructure:"boilerplateZipFile" usage:"boilerplate zip file to extract, if not provided, use embedded boilerplate"`
}

func InitializeConfig(flagSet *pflag.FlagSet, envPrefix string) (configx.Loader[*Config], error) {
	return configx.Initialize(flagSet, envPrefix, &Config{
		TargetDir:          ".",
		GoModule:           "",
		BoilerplateZipFile: "",
	})
}
