package sub

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
)

type yamlQuotedString string

func (s yamlQuotedString) MarshalYAML() ([]byte, error) {
	return []byte("'" + strings.ReplaceAll(string(s), "'", "''") + "'"), nil
}

var yamlNonStringWords = map[string]bool{
	"~": true, "null": true, "Null": true, "NULL": true,
	"true": true, "True": true, "TRUE": true,
	"false": true, "False": true, "FALSE": true,
	"yes": true, "Yes": true, "YES": true,
	"no": true, "No": true, "NO": true,
	"on": true, "On": true, "ON": true,
	"off": true, "Off": true, "OFF": true,
	"y": true, "Y": true, "n": true, "N": true,
}

var yamlNonStringNumber = regexp.MustCompile(`^(?:` +
	`[-+]?[0-9]+|` +
	`0[oO]?[0-7]+|0[xX][0-9a-fA-F]+|` +
	`[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+|` +
	`[-+]?(?:[0-9]*\.[0-9]+|[0-9]+\.?[0-9]*)(?:[eE][-+]?[0-9]+)?|` +
	`[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN)|` +
	`[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}(?:[Tt ].*)?` +
	`)$`)

func yamlScalarIsAmbiguous(s string) bool {
	if s == "" {
		return false
	}
	return yamlNonStringWords[s] || yamlNonStringNumber.MatchString(s)
}

// yamlEscapedString is written double-quoted on one line: the encoder's block scalars
// mishandle CR and the Unicode line breaks, so the text could continue as YAML.
type yamlEscapedString string

func (s yamlEscapedString) MarshalYAML() ([]byte, error) {
	return []byte(strconv.Quote(string(s))), nil
}

func yamlNeedsEscaping(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}
	return strings.ContainsFunc(s, func(r rune) bool { return !unicode.IsPrint(r) })
}

func quoteYAMLString(s string) any {
	switch {
	case yamlNeedsEscaping(s):
		return yamlEscapedString(s)
	case yamlScalarIsAmbiguous(s):
		return yamlQuotedString(s)
	}
	return s
}

func quoteAmbiguousYAMLScalars(v any) any {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		return quoteYAMLString(s)
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		escapedKeys := false
		iter := rv.MapRange()
		for iter.Next() {
			key, ok := iter.Key().Interface().(string)
			if !ok {
				return v
			}
			escapedKeys = escapedKeys || yamlNeedsEscaping(key)
			out[key] = quoteAmbiguousYAMLScalars(iter.Value().Interface())
		}
		if !escapedKeys {
			return out
		}
		// Only marshaler-typed keys escape, and only a map[any]any key keeps its type.
		keyed := make(map[any]any, len(out))
		for key, value := range out {
			if yamlNeedsEscaping(key) {
				keyed[yamlEscapedString(key)] = value
			} else {
				keyed[key] = value
			}
		}
		return keyed
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
			return v
		}
		out := make([]any, rv.Len())
		for i := range rv.Len() {
			out[i] = quoteAmbiguousYAMLScalars(rv.Index(i).Interface())
		}
		return out
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return v
		}
		return quoteAmbiguousYAMLScalars(rv.Elem().Interface())
	default:
		return v
	}
}

func marshalClashYAML(config any) ([]byte, error) {
	return yaml.Marshal(quoteAmbiguousYAMLScalars(config))
}
