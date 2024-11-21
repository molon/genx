package configx_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/molon/genx/pkg/configx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DatabaseConfig struct {
	Host string `mapstructure:"host" usage:"Database host" validate:"required"`
	Port int    `mapstructure:"port" usage:"Database port" validate:"gte=1,lte=65535"`
}

type JWTConfig struct {
	SigningAlgorithm string `mapstructure:"signingAlgorithm"`
	PrivateKey       string `mapstructure:"privateKey"`
	PublicKey        string `mapstructure:"publicKey"`
}

type AuthConfig struct {
	JWT                *JWTConfig    `mapstructure:"jwt"`
	AccessTokenExpiry  time.Duration `mapstructure:"accessTokenExpiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refreshTokenExpiry"`
}

type ExtraConfig struct {
	Int64          int64             `mapstructure:"int64"`
	Uint32         uint32            `mapstructure:"uint32"`
	Uint8          uint8             `mapstructure:"uint8"`
	Float64        float64           `mapstructure:"float64"`
	Bool           bool              `mapstructure:"bool"`
	StringSlice    []string          `mapstructure:"stringSlice"`
	IntSlice       []int             `mapstructure:"intSlice"`
	BoolSlice      []bool            `mapstructure:"boolSlice"`
	Float64Slice   []float64         `mapstructure:"float64Slice"`
	StringToString map[string]string `mapstructure:"stringToString"`
	StringToInt    map[string]int    `mapstructure:"stringToInt"`
	Time           time.Time         `mapstructure:"time"`
}

// TestConfig defines a configuration structure for testing purposes.
type TestConfig struct {
	Port         int            `mapstructure:"port" usage:"Port to run the server on" validate:"gte=1,lte=65535"`
	LogFiles     []string       `mapstructure:"logFiles" usage:"List of log files" validate:"dive,required"`
	RetryCounts  []int          `mapstructure:"retryCounts" usage:"List of retry counts" validate:"dive,gte=0"`
	Verbose      bool           `mapstructure:"verbose" usage:"Enable verbose logging"` // Missing tag
	MaxIdleConns int            `usage:"Maximum number of idle connections"`            // Missing mapstructure tag
	Timeout      time.Duration  `mapstructure:"timeout" usage:"Request timeout duration" validate:"gte=0"`
	Database     DatabaseConfig `mapstructure:"database" validate:"required"`
	Auth         *AuthConfig    `mapstructure:"auth"`
	Extra        ExtraConfig    `mapstructure:"extra"`
	Ignore       string         `mapstructure:"-"` // Ignored field
}

func TestInitializeLoader(t *testing.T) {
	viper.Reset()

	def := TestConfig{
		Port:         8080,
		LogFiles:     []string{"/var/log/app1.log", "/var/log/app2.log"},
		RetryCounts:  []int{3, 5},
		Verbose:      true,
		MaxIdleConns: 1,
		Timeout:      30 * time.Second,
		Database: DatabaseConfig{
			Host: "localhost",
			Port: 5432,
		},
	}

	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	loader, err := configx.Initialize(flagSet, "APP_", def)
	require.NoError(t, err)

	// Create a temporary configuration file
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configFileContent := `
port: 6060
logFiles:
  - "/var/log/app3.log"
  - "/var/log/app4.log"
retryCounts:
  - 33
  - 55
# verbose: false
MaxIdleConns: 10
timeout: "33s"
database:
  host: "localhost"
  port: 5433
ignore: "ignored" # will be ignored
`
	err = os.WriteFile(configFilePath, []byte(configFileContent), 0o644)
	require.NoError(t, err)

	// Set environment variables
	os.Setenv("APP_PORT", "9090")
	os.Setenv("APP_LOG_FILES", "/tmp/app1.log,/tmp/app2.log")
	os.Setenv("APP_DATABASE_HOST", "127.0.0.1")
	os.Setenv("APP_DATABASE_PORT", "3306")
	os.Setenv("APP_EXTRA_STRING_SLICE", "a,b,c")
	os.Setenv("APP_EXTRA_INT_SLICE", "1,2")
	defer func() {
		os.Unsetenv("APP_PORT")
		os.Unsetenv("APP_LOG_FILES")
		os.Unsetenv("APP_DATABASE_HOST")
		os.Unsetenv("APP_DATABASE_PORT")
		os.Unsetenv("APP_EXTRA_STRING_SLICE")
		os.Unsetenv("APP_EXTRA_INT_SLICE")
	}()

	// Simulate command-line arguments
	args := []string{
		"--port=7070",
		"--retry-counts=4", "--retry-counts=6",
		"--log-files=/tmp/app3.log", "--log-files=/tmp/app4.log",
		"--extra-bool-slice=true", "--extra-bool-slice=false",
		"--extra-float64-slice=1.1", "--extra-float64-slice=2.2",
	}
	err = flagSet.Parse(args)
	require.NoError(t, err)

	// Load configuration with the config file path
	config, err := loader(configFilePath)
	require.NoError(t, err)

	expected := TestConfig{
		Port:         7070,                                       // flag
		LogFiles:     []string{"/tmp/app3.log", "/tmp/app4.log"}, // flag
		RetryCounts:  []int{4, 6},                                // flag
		Verbose:      true,                                       // default
		MaxIdleConns: 10,                                         // yaml
		Timeout:      33 * time.Second,                           // yaml
		Database: DatabaseConfig{
			Host: "127.0.0.1", // env
			Port: 3306,        // env
		},
		Auth: &AuthConfig{
			JWT: &JWTConfig{},
		},
		Extra: ExtraConfig{
			Uint32:         0,
			Uint8:          0,
			Float64:        0,
			Bool:           false,
			StringSlice:    []string{"a", "b", "c"}, // env
			IntSlice:       []int{1, 2},             // env
			BoolSlice:      []bool{true, false},     // flag
			Float64Slice:   []float64{1.1, 2.2},     // flag
			StringToString: map[string]string{},
			StringToInt:    map[string]int{},
		},
		Ignore: "", // Ignored
	}
	assert.Equal(t, expected, config)
}

func TestInitializeLoaderValidationFailure(t *testing.T) {
	viper.Reset()

	def := &TestConfig{
		Port:         8080,
		LogFiles:     []string{"/var/log/app1.log", ""}, // Invalid: empty string
		RetryCounts:  []int{-1, 5},                      // Invalid: negative number
		Verbose:      false,
		MaxIdleConns: 1,
		Timeout:      -10 * time.Second, // Invalid: negative duration
		Database: DatabaseConfig{
			Host: "",    // Invalid: required
			Port: 70000, // Invalid: exceeds max
		},
	}

	flagSet := pflag.NewFlagSet("test_validation_failure", pflag.ContinueOnError)
	loader, err := configx.Initialize(flagSet, "APP_", def)
	require.NoError(t, err)

	// Load configuration without specifying a config file path
	config, err := loader("")
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestWithFieldHook(t *testing.T) {
	viper.Reset()

	def := &TestConfig{
		Port:         8080,
		LogFiles:     []string{"/var/log/app1.log", "/var/log/app2.log"},
		RetryCounts:  []int{3, 5},
		Verbose:      true,
		MaxIdleConns: 1,
		Timeout:      30 * time.Second,
		Database: DatabaseConfig{
			Host: "localhost",
			Port: 5432,
		},
	}

	flagSet := pflag.NewFlagSet("test_field_hook", pflag.ContinueOnError)
	loader, err := configx.Initialize(
		flagSet, "APP_", def,
		configx.WithFieldHook(func(viperKey, flagKey, envKey, usage string) (string, string, string, string) {
			if viperKey == "port" {
				envKey = "APP_HTTP_PORT"
			}
			if viperKey == "MaxIdleConns" {
				assert.Equal(t, "max-idle-conns", flagKey)
				assert.Equal(t, "APP_MAX_IDLE_CONNS", envKey)
				assert.Equal(t, "Maximum number of idle connections", usage)
				// t.Logf("viperKey: %s, flagKey: %s, envKey: %s, usage: %s", viperKey, flagKey, envKey, usage)
			}
			return viperKey, flagKey, envKey, usage
		}),
	)
	require.NoError(t, err)

	os.Setenv("APP_PORT", "9090")
	os.Setenv("APP_HTTP_PORT", "7070")
	defer func() {
		os.Unsetenv("APP_PORT")
		os.Unsetenv("APP_HTTP_PORT")
	}()

	config, err := loader("")
	require.NoError(t, err)

	assert.Equal(t, 7070, config.Port)
}

func TestRead(t *testing.T) {
	configFileContent := `
port: 6060
logFiles:
  - "/var/log/app3.log"
  - "/var/log/app4.log"
retryCounts:
  - 33
  - 55
# verbose: true
MaxIdleConns: 10
timeout: "33s"
database:
  host: "localhost"
  port: 5433
extra:
  stringSlice: "a,b,c"
  intSlice: "1,2"
ignore: "ignored" # will be ignored
`
	conf, err := configx.Read[*TestConfig]("yaml", strings.NewReader(configFileContent))
	require.NoError(t, err)

	expected := &TestConfig{
		Port:         6060,
		LogFiles:     []string{"/var/log/app3.log", "/var/log/app4.log"},
		RetryCounts:  []int{33, 55},
		Verbose:      false,
		MaxIdleConns: 10,
		Timeout:      33 * time.Second,
		Database: DatabaseConfig{
			Host: "localhost",
			Port: 5433,
		},
		Extra: ExtraConfig{
			StringSlice: []string{"a", "b", "c"},
			IntSlice:    []int{1, 2},
		},
		Ignore: "",
	}
	assert.Equal(t, expected, conf)
}
