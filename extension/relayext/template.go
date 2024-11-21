package relayext

import (
	"strings"
	"text/template"

	"github.com/jinzhu/inflection"
	"github.com/samber/lo"
)

var Funcs = template.FuncMap{
	"toLower":       strings.ToLower,
	"toUpper":       strings.ToUpper,
	"trim":          strings.Trim,
	"trimSuffix":    strings.TrimSuffix,
	"hasPrefix":     strings.HasPrefix,
	"hasSuffix":     strings.HasSuffix,
	"replaceAll":    strings.ReplaceAll,
	"split":         strings.Split,
	"camelCase":     lo.CamelCase,
	"snakeCase":     lo.SnakeCase,
	"pascalCase":    lo.PascalCase,
	"kebabCase":     lo.KebabCase,
	"capitalize":    lo.Capitalize,
	"plural":        inflection.Plural,
	"singular":      inflection.Singular,
	"typeString":    TypeString,
	"isPointerType": IsPointerType,
}
