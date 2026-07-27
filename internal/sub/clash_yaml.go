package sub

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// yamlQuotedString is a string that must reach a YAML parser as a string.
// Single-quoted style is used because it carries the value literally — no
// escape processing — which suits hex ids, passwords and pre-shared keys.
type yamlQuotedString string

func (s yamlQuotedString) MarshalYAML() ([]byte, error) {
	return []byte("'" + strings.ReplaceAll(string(s), "'", "''") + "'"), nil
}

// yamlPlainScalarNotString matches the plain scalars a YAML parser resolves to
// something other than a string. It covers the YAML 1.2 core schema plus the
// 1.1 forms go-yaml v3 — the library the Clash cores use — still resolves:
// null, booleans (including y/yes/on and friends), integers in every base,
// sexagesimals, floats, and dates.
var yamlPlainScalarNotString = regexp.MustCompile(`^(?:` +
	`~|[Nn]ull|NULL|` +
	`[Tt]rue|TRUE|[Ff]alse|FALSE|[Yy]es|YES|[Nn]o|NO|[Oo]n|ON|[Oo]ff|OFF|[YyNn]|` +
	`[-+]?[0-9]+|` +
	`0[oO]?[0-7]+|0[xX][0-9a-fA-F]+|` +
	`[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+|` +
	`[-+]?(?:[0-9]*\.[0-9]+|[0-9]+\.?[0-9]*)(?:[eE][-+]?[0-9]+)?|` +
	`[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN)|` +
	`[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}(?:[Tt ].*)?` +
	`)$`)

// yamlScalarIsAmbiguous reports whether s must be quoted to survive as a string.
//
// The encoder quotes most of these itself, but not the exponent-float form: a
// REALITY short-id such as "2351e1" is valid hex and, emitted bare, is read as
// 23510. A Clash core then hex-decodes a five-digit number, rejects the proxy
// and drops the whole provider to zero nodes (#6104). goccy's own parser reads
// that token back as a string, so the encoder never sees a problem and a
// round-trip check through it cannot find one either — the resolution rules,
// not the encoder, are the thing to test against. The same shape reaches
// passwords, obfs-passwords and pre-shared keys, so every string in the
// document is checked rather than an enumerated list of fields.
func yamlScalarIsAmbiguous(s string) bool {
	if s == "" {
		return false
	}
	return yamlPlainScalarNotString.MatchString(s)
}

// quoteAmbiguousYAMLScalars rebuilds v with every ambiguous string wrapped so
// the encoder is forced to quote it. Containers are rebuilt as map[string]any /
// []any because a typed container cannot hold the wrapper; unambiguous values
// are carried through untouched, so the emitted document is unchanged apart
// from the quotes that were missing.
func quoteAmbiguousYAMLScalars(v any) any {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		if yamlScalarIsAmbiguous(s) {
			return yamlQuotedString(s)
		}
		return s
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			key, ok := iter.Key().Interface().(string)
			if !ok {
				return v
			}
			out[key] = quoteAmbiguousYAMLScalars(iter.Value().Interface())
		}
		return out
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
			return v
		}
		out := make([]any, rv.Len())
		for i := range rv.Len() {
			out[i] = quoteAmbiguousYAMLScalars(rv.Index(i).Interface())
		}
		return out
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return v
		}
		return quoteAmbiguousYAMLScalars(rv.Elem().Interface())
	default:
		return v
	}
}

// marshalClashYAML serializes a Clash config, keeping string values typed as
// strings in the output.
func marshalClashYAML(config any) ([]byte, error) {
	return yaml.Marshal(quoteAmbiguousYAMLScalars(config))
}
