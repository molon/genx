package starter

import (
	"strings"

	_ "embed"

	"github.com/molon/genx/pkg/configx"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

type Config struct {
	TargetDir          string `mapstructure:"targetDir" usage:"target directory to extract boilerplate"`
	GoModule           string `mapstructure:"goModule" usage:"go module path to replace in boilerplate"`
	BoilerplateZipFile string `mapstructure:"boilerplateZipFile" usage:"boilerplate zip file to extract, if not provided, use embedded boilerplate"`
}

//go:embed embed/default.yaml
var defaultConfigYAML string

func InitializeConfig(flagSet *pflag.FlagSet, envPrefix string) (configx.Loader[*Config], error) {
	def, err := configx.Read[*Config]("yaml", strings.NewReader(defaultConfigYAML))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load default config from embedded YAML")
	}
	return configx.Initialize(flagSet, envPrefix, def)
}
