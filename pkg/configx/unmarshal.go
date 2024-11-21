package configx

import (
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var DecoderConfigOption = func(dc *mapstructure.DecoderConfig) {
	dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToTimeHookFunc(time.RFC3339),
		StringToSliceHookFunc(","),
	)
	// dc.TagName = "mapstructure"
}

func Read[T any](typ string, r io.Reader) (T, error) {
	var zero T

	viperInstance := viper.New()
	viperInstance.SetConfigType(strings.TrimLeft(typ, "."))
	if err := viperInstance.ReadConfig(r); err != nil {
		return zero, errors.Wrap(err, "failed to read config")
	}

	var def T
	if err := viperInstance.Unmarshal(&def, DecoderConfigOption); err != nil {
		return zero, errors.Wrap(err, "failed to unmarshal config")
	}

	return def, nil
}

func StringToSliceHookFunc(separator string) mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() == reflect.String && to.Kind() == reflect.Slice {
			str := strings.Trim(data.(string), "[]")
			parts := strings.Split(str, separator)

			elemType := to.Elem()
			switch elemType.Kind() {
			case reflect.String:
				return parts, nil
			case reflect.Int:
				return parseSlice(parts, strconv.Atoi)
			case reflect.Bool:
				return parseSlice(parts, strconv.ParseBool)
			case reflect.Float64:
				return parseSlice(parts, func(s string) (float64, error) {
					return strconv.ParseFloat(s, 64)
				})
			default:
				return nil, errors.Errorf("unsupported slice element type %s", elemType)
			}
		}
		return data, nil
	}
}

func parseSlice[T any](parts []string, parseFunc func(string) (T, error)) ([]T, error) {
	if len(parts) == 0 {
		return []T{}, nil
	}
	if len(parts) == 1 && strings.TrimSpace(parts[0]) == "" {
		return []T{}, nil
	}
	result := make([]T, len(parts))
	for i, part := range parts {
		parsed, err := parseFunc(strings.TrimSpace(part))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value %q", part)
		}
		result[i] = parsed
	}
	return result, nil
}
