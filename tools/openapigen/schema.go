package main

import (
	"reflect"
	"sort"
	"strings"
)

type Schema struct {
	Name    string
	Package string
	Fields  []Field
	Doc     string
}

type Alias struct {
	Name       string
	Package    string
	Underlying TypeRef
}

type Field struct {
	JSONName string
	GoName   string
	Type     TypeRef
	Optional bool
	Skip     bool
	Validate []ValidateRule
	Doc      string
}

type TypeRef struct {
	Kind    TypeKind
	Name    string
	Element *TypeRef
	Key     *TypeRef
	Value   *TypeRef
	Inner   *TypeRef
}

type TypeKind string

const (
	KindString  TypeKind = "string"
	KindNumber  TypeKind = "number"
	KindInt     TypeKind = "int"
	KindBool    TypeKind = "boolean"
	KindArray   TypeKind = "array"
	KindMap     TypeKind = "map"
	KindObject  TypeKind = "object"
	KindRef     TypeKind = "ref"
	KindUnknown TypeKind = "unknown"
	KindAny     TypeKind = "any"
	KindRaw     TypeKind = "raw"
)

type ValidateRule struct {
	Name  string
	Param string
}

func parseStructTag(raw string) (json string, validate string, gormHasDash bool) {
	tag := reflect.StructTag(strings.Trim(raw, "`"))
	json = tag.Get("json")
	validate = tag.Get("validate")
	if g := tag.Get("gorm"); g != "" {
		for part := range strings.SplitSeq(g, ";") {
			if strings.TrimSpace(part) == "-" {
				gormHasDash = true
			}
		}
	}
	return
}

func parseJSONTag(tag string) (name string, omit bool, omitempty bool) {
	if tag == "" {
		return "", false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "-" {
		return "", true, false
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
		}
	}
	return
}

func parseValidateTag(tag string) []ValidateRule {
	if tag == "" {
		return nil
	}
	var rules []ValidateRule
	for part := range strings.SplitSeq(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		before, after, ok := strings.Cut(part, "=")
		if !ok {
			rules = append(rules, ValidateRule{Name: part})
			continue
		}
		rules = append(rules, ValidateRule{Name: before, Param: after})
	}
	return rules
}

func (s Schema) HasValidationOn(field string) bool {
	for _, f := range s.Fields {
		if f.JSONName == field {
			return len(f.Validate) > 0
		}
	}
	return false
}

func sortSchemas(in []Schema) []Schema {
	out := make([]Schema, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func sortAliases(in []Alias) []Alias {
	out := make([]Alias, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func flattenEmbedded(schemas []Schema) []Schema {
	byName := make(map[string]Schema, len(schemas))
	for _, s := range schemas {
		byName[s.Name] = s
	}
	out := make([]Schema, 0, len(schemas))
	for _, s := range schemas {
		var resolved []Field
		seen := make(map[string]bool, len(s.Fields))
		for _, f := range s.Fields {
			if f.Type.Kind == KindRef && f.Type.Name != "nullable" {
				if embedded, ok := byName[f.Type.Name]; ok && f.GoName == f.Type.Name {
					for _, ef := range embedded.Fields {
						if seen[ef.JSONName] {
							continue
						}
						seen[ef.JSONName] = true
						resolved = append(resolved, ef)
					}
					continue
				}
			}
			if seen[f.JSONName] {
				continue
			}
			seen[f.JSONName] = true
			resolved = append(resolved, f)
		}
		s.Fields = resolved
		out = append(out, s)
	}
	return out
}
