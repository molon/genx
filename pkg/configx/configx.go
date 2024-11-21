package configx

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/huandu/go-clone"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Loader[T any] func(path string) (T, error)

// Initialize sets up configuration binding by automatically registering command-line flags,
// binding environment variables, loading configuration files, and validating the final configuration.
//
// It leverages reflection to traverse the fields of the provided default configuration struct,
// supports various data types including basic types, slices, maps, and nested structs defined separately,
// and integrates with Viper for configuration management and go-playground/validator for validation.
//
// Parameters:
//   - flagSet: A pointer to a pflag.FlagSet used for registering command-line flags.
//     If nil, the default pflag.CommandLine is used.
//   - envPrefix: A string prefix for environment variables. Environment variables will be
//     bound using this prefix followed by the flag name in uppercase with
//     dots and hyphens replaced by underscores.
//   - def: The default configuration struct.
//
// Returns:
//   - Loader[T]: A generic loader function that accepts an optional configuration file path.
//     When invoked, it parses the command-line flags, binds them along with environment
//     variables, loads the configuration file if provided, unmarshals the configuration
//     into the struct, and validates it.
//   - error: An error object if initialization fails.
//
// Usage Example:
//
//	type DatabaseConfig struct {
//	    Host string `mapstructure:"host" usage:"Database host" validate:"required"`
//	    Port int    `mapstructure:"port" usage:"Database port" validate:"gte=1,lte=65535"`
//	}
//
//	type Config struct {
//	    Port        int            `mapstructure:"port" usage:"Port to run the server on" validate:"gte=1,lte=65535"`
//	    LogFiles    []string       `mapstructure:"logFiles" usage:"List of log files" validate:"required,dive,required"`
//	    RetryCounts []int          `mapstructure:"retryCounts" usage:"List of retry counts" validate:"dive,gte=0"`
//	    Verbose     bool           `mapstructure:"verbose" usage:"Enable verbose logging"`
//	    Timeout     time.Duration  `mapstructure:"timeout" usage:"Request timeout duration" validate:"gte=0"`
//	    Database    DatabaseConfig `mapstructure:"database" validate:"required"`
//	}
//
//	func main() {
//	    // Define the default configuration
//	    def := Config{
//	        Port:        8080,
//	        LogFiles:    []string{"/var/log/app1.log", "/var/log/app2.log"},
//	        RetryCounts: []int{3, 5},
//	        Verbose:     false,
//	        Timeout:     30 * time.Second,
//	        Database: DatabaseConfig{
//	            Host: "localhost",
//	            Port: 5432,
//	        },
//	    }
//
//	    // Initialize the loader with a custom FlagSet, environment variable prefix, and default config
//	    loader, err := Initialize(&pflag.CommandLine, "APP_", def)
//	    if err != nil {
//	        fmt.Println("Error initializing config:", err)
//	        return
//	    }
//
//	    // Optionally, create a configuration file and pass its path to the loader
//	    config, err := loader("config.yaml")
//	    if err != nil {
//	        fmt.Println("Error loading config:", err)
//	        return
//	    }
//
//	    // Use the loaded and validated configuration
//	    fmt.Printf("Final Config: %+v\n", config)
//	}
func Initialize[T any](flagSet *pflag.FlagSet, envPrefix string, def T, opts ...Option) (Loader[T], error) {
	options := &initOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if flagSet == nil {
		flagSet = pflag.CommandLine
	}
	def = clone.Slowly(def).(T)

	collectBinds := []func() error{}
	err := initializeRecursive(flagSet, envPrefix, reflect.ValueOf(def), "", &collectBinds, options.fieldHook)
	if err != nil {
		return nil, err
	}
	var once sync.Once
	var bindErr error
	return func(path string) (T, error) {
		once.Do(func() {
			if !flagSet.Parsed() {
				if err := flagSet.Parse(os.Args[1:]); err != nil {
					bindErr = errors.Wrap(err, "failed to parse flags")
					return
				}
			}

			for _, bind := range collectBinds {
				if err := bind(); err != nil {
					bindErr = err
					return
				}
			}
		})
		var zero T
		if bindErr != nil {
			return zero, bindErr
		}

		if path != "" {
			viper.SetConfigFile(path)
			if err := viper.ReadInConfig(); err != nil {
				return zero, errors.Wrapf(err, "failed to read config %s", path)
			}
		}

		var config T
		if err := viper.Unmarshal(&config, DecoderConfigOption); err != nil {
			return zero, errors.Wrapf(err, "failed to unmarshal config to %T", zero)
		}

		validatorx := options.validator
		if validatorx == nil {
			validatorx = validator.New(validator.WithRequiredStructEnabled())
		}
		if err := validatorx.Struct(config); err != nil {
			return zero, errors.Wrap(err, "validation failed for config")
		}

		return config, nil
	}, nil
}

var envReplacer = strings.NewReplacer(".", "_", "-", "_")

func indirectOrNew(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.New(v.Type().Elem()).Elem()
		}
		v = v.Elem()
	}
	return v
}

var reKebabCaseFixDigital = regexp.MustCompile(`-(\d+)`)

func initializeRecursive(
	flagSet *pflag.FlagSet,
	envPrefix string,
	v reflect.Value,
	parentKey string,
	collectBinds *[]func() error,
	fieldHook func(viperKey, flagKey, envKey, usage string) (string, string, string, string),
) error {
	v = indirectOrNew(v)

	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		fieldValue := indirectOrNew(v.Field(i))
		tag := strings.TrimSpace(field.Tag.Get("mapstructure"))
		if tag == "" {
			tag = field.Name
		}
		if tag == "-" {
			continue
		}

		viperKey := tag
		if parentKey != "" {
			viperKey = parentKey + "." + viperKey
		}
		flagKey := reKebabCaseFixDigital.ReplaceAllString(lo.KebabCase(viperKey), "${1}")
		envKey := envPrefix + envReplacer.Replace(strings.ToUpper(flagKey))
		usage := strings.TrimSpace(field.Tag.Get("usage"))
		if usage == "" {
			usage = viperKey // Fallback to viperKey if usage is not provided
		}

		if fieldHook != nil {
			viperKey, flagKey, envKey, usage = fieldHook(viperKey, flagKey, envKey, usage)
		}

		defaultValue := fieldValue.Interface()
		switch fieldValue.Kind() {
		case reflect.String:
			flagSet.String(flagKey, defaultValue.(string), usage)
		case reflect.Int:
			flagSet.Int(flagKey, defaultValue.(int), usage)
		case reflect.Int64:
			if fieldValue.Type() == reflect.TypeOf(time.Duration(0)) {
				flagSet.Duration(flagKey, defaultValue.(time.Duration), usage)
			} else {
				flagSet.Int64(flagKey, defaultValue.(int64), usage)
			}
		case reflect.Uint32:
			flagSet.Uint32(flagKey, defaultValue.(uint32), usage)
		case reflect.Uint8:
			flagSet.Uint8(flagKey, defaultValue.(uint8), usage)
		case reflect.Float64:
			flagSet.Float64(flagKey, defaultValue.(float64), usage)
		case reflect.Bool:
			flagSet.Bool(flagKey, defaultValue.(bool), usage)
		case reflect.Slice:
			elemKind := fieldValue.Type().Elem().Kind()
			switch elemKind {
			case reflect.String:
				flagSet.StringSlice(flagKey, defaultValue.([]string), usage)
			case reflect.Int:
				flagSet.IntSlice(flagKey, defaultValue.([]int), usage)
			case reflect.Bool:
				flagSet.BoolSlice(flagKey, defaultValue.([]bool), usage)
			case reflect.Float64:
				flagSet.Float64Slice(flagKey, defaultValue.([]float64), usage)
			default:
				return errors.Errorf("unsupported slice element type: %s for key %s", elemKind, viperKey)
			}
		case reflect.Map:
			if fieldValue.Type().Key().Kind() == reflect.String && fieldValue.Type().Elem().Kind() == reflect.String {
				flagSet.StringToString(flagKey, defaultValue.(map[string]string), usage)
			} else if fieldValue.Type().Key().Kind() == reflect.String && fieldValue.Type().Elem().Kind() == reflect.Int {
				flagSet.StringToInt(flagKey, defaultValue.(map[string]int), usage)
			} else {
				return errors.Errorf("unsupported map key/value types for key %s", viperKey)
			}
		case reflect.Struct:
			if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
				flagSet.String(flagKey, defaultValue.(time.Time).Format(time.RFC3339), usage+" (time in RFC3339 format)")
			} else {
				if err := initializeRecursive(flagSet, envPrefix, fieldValue, viperKey, collectBinds, fieldHook); err != nil {
					return err
				}
				continue
			}
		default:
			return errors.Errorf("unsupported field type: %s for key %s", fieldValue.Kind(), viperKey)
		}

		*collectBinds = append(*collectBinds, func() error {
			if err := viper.BindPFlag(viperKey, flagSet.Lookup(flagKey)); err != nil {
				return errors.Wrapf(err, "failed to bind flag %s", flagKey)
			}
			return nil
		})

		*collectBinds = append(*collectBinds, func() error {
			if err := viper.BindEnv(viperKey, envKey); err != nil {
				return errors.Wrapf(err, "failed to bind env %s", envKey)
			}
			return nil
		})
	}

	return nil
}

type Option func(opts *initOptions)

type initOptions struct {
	validator *validator.Validate
	fieldHook func(viperKey, flagKey, envKey, usage string) (string, string, string, string)
}

// WithFieldHook sets a custom field hook function that maps configuration field names to Viper keys, flag names, environment variable names and usage strings.
func WithFieldHook(hook func(viperKey, flagKey, envKey, usage string) (string, string, string, string)) Option {
	return func(opts *initOptions) {
		opts.fieldHook = hook
	}
}

// WithValidator sets a custom validator instance for validating the configuration struct.
func WithValidator(v *validator.Validate) Option {
	return func(opts *initOptions) {
		opts.validator = v
	}
}
