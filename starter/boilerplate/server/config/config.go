package config

import (
	_ "embed"
	"strings"
	"time"

	"github.com/molon/genx/pkg/configx"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

type DatabaseConfig struct {
	DSN             string        `mapstructure:"dsn" usage:"Database DSN"`
	Debug           bool          `mapstructure:"debug" usage:"Enable database debug mode"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns" usage:"Database max idle connections"`
	MaxOpenConns    int           `mapstructure:"maxOpenConns" usage:"Database max open connections"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime" usage:"Database connection max lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"connMaxIdleTime" usage:"Database connection max idle time"`
}

type ServerConfig struct {
	AllowedOrigins     []string `mapstructure:"allowedOrigins" usage:"CROS Allowed origins"`
	HTTPAddress        string   `mapstructure:"httpAddress" usage:"HTTP server address"`
	GraphQLEndpoint    string   `mapstructure:"graphqlEndpoint" usage:"GraphQL endpoint"`
	PlaygroundEndpoint string   `mapstructure:"playgroundEndpoint" usage:"GraphQL playground endpoint"`
}

type Config struct {
	DevMode  bool           `mapstructure:"devMode" usage:"Enable development mode"`
	Database DatabaseConfig `mapstructure:"database"`
	Server   ServerConfig   `mapstructure:"server"`
}

//go:embed embed/default.yaml
var defaultConfigYAML string

func Initialize(flagSet *pflag.FlagSet, envPrefix string) (configx.Loader[*Config], error) {
	def, err := configx.Read[*Config]("yaml", strings.NewReader(defaultConfigYAML))
	if err != nil {
		return nil, errors.Wrap(err, "failed to load default config from embedded YAML")
	}
	return configx.Initialize(flagSet, envPrefix, def)
}
