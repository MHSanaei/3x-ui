package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

func emitJSONSchema(w io.Writer, schemas []Schema, aliases []Alias) error {
	byName := make(map[string]Schema, len(schemas))
	for _, s := range schemas {
		byName[s.Name] = s
	}
	aliasByName := make(map[string]Alias, len(aliases))
	for _, a := range aliases {
		aliasByName[a.Name] = a
	}

	gen := &schemaGen{byName: byName, aliasByName: aliasByName}

	out := make(map[string]any, len(schemas))
	for _, s := range schemas {
		out[s.Name] = gen.objectSchema(s)
	}

	payload, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, examplesHeader); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "export const SCHEMAS: Record<string, unknown> = %s;\n", payload); err != nil {
		return err
	}
	return nil
}

type schemaGen struct {
	byName      map[string]Schema
	aliasByName map[string]Alias
}

func (g *schemaGen) objectSchema(s Schema) map[string]any {
	props := make(map[string]any, len(s.Fields))
	var required []string
	for _, f := range s.Fields {
		props[f.JSONName] = g.fieldSchema(f)
		if !f.Optional {
			required = append(required, f.JSONName)
		}
	}
	obj := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		sort.Strings(required)
		obj["required"] = required
	}
	if s.Doc != "" {
		obj["description"] = s.Doc
	}
	return obj
}

func (g *schemaGen) fieldSchema(f Field) map[string]any {
	sch := g.typeSchema(f.Type)
	if ref, ok := sch["$ref"]; ok {
		if f.Doc == "" && f.Example == "" {
			return sch
		}
		wrap := map[string]any{"allOf": []any{map[string]any{"$ref": ref}}}
		if f.Doc != "" {
			wrap["description"] = f.Doc
		}
		if f.Example != "" {
			wrap["example"] = coerceExample(f.Example, baseKind(f.Type))
		}
		return wrap
	}
	applyConstraints(sch, f.Type, f.Validate)
	if f.Doc != "" {
		sch["description"] = f.Doc
	}
	if f.Example != "" {
		sch["example"] = coerceExample(f.Example, baseKind(f.Type))
	}
	return sch
}

func (g *schemaGen) typeSchema(t TypeRef) map[string]any {
	switch t.Kind {
	case KindString:
		if t.Name == "datetime" {
			return map[string]any{"type": "string", "format": "date-time"}
		}
		return map[string]any{"type": "string"}
	case KindInt:
		sch := map[string]any{"type": "integer"}
		if t.Name == "int64" {
			sch["format"] = "int64"
		}
		return sch
	case KindNumber:
		return map[string]any{"type": "number"}
	case KindBool:
		return map[string]any{"type": "boolean"}
	case KindArray:
		return map[string]any{"type": "array", "items": g.typeSchema(*t.Element)}
	case KindMap:
		return map[string]any{"type": "object", "additionalProperties": g.typeSchema(*t.Value)}
	case KindAny, KindUnknown, KindRaw:
		return map[string]any{}
	case KindRef:
		if t.Name == "nullable" {
			inner := g.typeSchema(*t.Inner)
			if ref, ok := inner["$ref"]; ok {
				return map[string]any{"nullable": true, "allOf": []any{map[string]any{"$ref": ref}}}
			}
			inner["nullable"] = true
			return inner
		}
		if alias, ok := g.aliasByName[t.Name]; ok {
			return g.typeSchema(alias.Underlying)
		}
		if _, ok := g.byName[t.Name]; ok {
			return map[string]any{"$ref": "#/components/schemas/" + t.Name}
		}
		return map[string]any{}
	}
	return map[string]any{}
}

func applyConstraints(sch map[string]any, t TypeRef, rules []ValidateRule) {
	base := baseKind(t)
	numeric := base.Kind == KindInt || base.Kind == KindNumber
	str := base.Kind == KindString
	for _, r := range rules {
		switch r.Name {
		case "gte":
			if numeric {
				sch["minimum"] = coerceExample(r.Param, base)
			}
		case "lte":
			if numeric {
				sch["maximum"] = coerceExample(r.Param, base)
			}
		case "gt":
			if numeric {
				sch["minimum"] = coerceExample(r.Param, base)
				sch["exclusiveMinimum"] = true
			}
		case "lt":
			if numeric {
				sch["maximum"] = coerceExample(r.Param, base)
				sch["exclusiveMaximum"] = true
			}
		case "min":
			if numeric {
				sch["minimum"] = coerceExample(r.Param, base)
			} else if str {
				if n, err := strconv.Atoi(r.Param); err == nil {
					sch["minLength"] = n
				}
			}
		case "max":
			if numeric {
				sch["maximum"] = coerceExample(r.Param, base)
			} else if str {
				if n, err := strconv.Atoi(r.Param); err == nil {
					sch["maxLength"] = n
				}
			}
		case "oneof":
			vals := strings.Fields(r.Param)
			if len(vals) > 0 {
				enum := make([]any, len(vals))
				for i, v := range vals {
					enum[i] = v
				}
				sch["enum"] = enum
			}
		case "email":
			if str {
				sch["format"] = "email"
			}
		case "url":
			if str {
				sch["format"] = "uri"
			}
		}
	}
}
