package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

type walkOverride struct {
	Field string
	Kind  TypeKind
}

type packageRequest struct {
	Path        string
	StructAllow map[string]bool
	AliasAllow  map[string]bool
	Overrides   map[string][]walkOverride
}

func walkPackages(requests []packageRequest) ([]Schema, []Alias, error) {
	fset := token.NewFileSet()
	var schemas []Schema
	var aliases []Alias
	for _, req := range requests {
		dir := req.Path
		pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ParseComments)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", dir, err)
		}
		for _, pkg := range pkgs {
			for _, file := range pkg.Files {
				for _, decl := range file.Decls {
					gen, ok := decl.(*ast.GenDecl)
					if !ok || gen.Tok != token.TYPE {
						continue
					}
					for _, spec := range gen.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						if strct, ok := ts.Type.(*ast.StructType); ok {
							if req.StructAllow != nil && !req.StructAllow[ts.Name.Name] {
								continue
							}
							s := Schema{
								Name:    ts.Name.Name,
								Package: pkg.Name,
								Doc:     collectDoc(gen.Doc, ts.Doc),
							}
							overrides := req.Overrides[ts.Name.Name]
							for _, fld := range strct.Fields.List {
								for _, f := range buildFields(fld, overrides) {
									s.Fields = append(s.Fields, f)
								}
							}
							schemas = append(schemas, s)
							continue
						}
						if req.AliasAllow != nil && !req.AliasAllow[ts.Name.Name] {
							continue
						}
						aliases = append(aliases, Alias{
							Name:       ts.Name.Name,
							Package:    pkg.Name,
							Underlying: exprToType(ts.Type),
						})
					}
				}
			}
		}
	}
	return schemas, aliases, nil
}

func collectDoc(group ...*ast.CommentGroup) string {
	var b strings.Builder
	for _, g := range group {
		if g == nil {
			continue
		}
		for _, c := range g.List {
			line := strings.TrimPrefix(c.Text, "// ")
			line = strings.TrimPrefix(line, "//")
			b.WriteString(strings.TrimSpace(line))
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

func buildFields(fld *ast.Field, overrides []walkOverride) []Field {
	var fields []Field
	tag := ""
	if fld.Tag != nil {
		tag = fld.Tag.Value
	}
	jsonTag, validateTag, gormDash := parseStructTag(tag)
	if gormDash && jsonTag == "" {
		return nil
	}
	jsonName, omit, omitempty := parseJSONTag(jsonTag)
	if omit {
		return nil
	}
	validate := parseValidateTag(validateTag)
	doc := collectDoc(fld.Doc, fld.Comment)

	for _, n := range fld.Names {
		fname := jsonName
		if fname == "" {
			fname = lowerFirst(n.Name)
		}
		t := exprToType(fld.Type)
		for _, o := range overrides {
			if o.Field == n.Name || o.Field == jsonName {
				t = TypeRef{Kind: o.Kind}
				break
			}
		}
		fields = append(fields, Field{
			JSONName: fname,
			GoName:   n.Name,
			Type:     t,
			Optional: omitempty || isPointer(fld.Type),
			Validate: validate,
			Doc:      doc,
		})
	}

	if len(fld.Names) == 0 {
		fname := jsonName
		if fname == "" {
			fname = lowerFirst(exprIdentName(fld.Type))
		}
		t := exprToType(fld.Type)
		for _, o := range overrides {
			if o.Field == exprIdentName(fld.Type) || o.Field == jsonName {
				t = TypeRef{Kind: o.Kind}
				break
			}
		}
		fields = append(fields, Field{
			JSONName: fname,
			GoName:   exprIdentName(fld.Type),
			Type:     t,
			Optional: omitempty || isPointer(fld.Type),
			Validate: validate,
			Doc:      doc,
		})
	}

	return fields
}

func exprToType(expr ast.Expr) TypeRef {
	switch e := expr.(type) {
	case *ast.Ident:
		return identType(e.Name)
	case *ast.StarExpr:
		inner := exprToType(e.X)
		return TypeRef{Kind: KindRef, Name: "nullable", Inner: &inner}
	case *ast.ArrayType:
		elem := exprToType(e.Elt)
		return TypeRef{Kind: KindArray, Element: &elem}
	case *ast.MapType:
		k := exprToType(e.Key)
		v := exprToType(e.Value)
		return TypeRef{Kind: KindMap, Key: &k, Value: &v}
	case *ast.SelectorExpr:
		pkg := exprIdentName(e.X)
		name := e.Sel.Name
		if pkg == "json" && name == "RawMessage" {
			return TypeRef{Kind: KindAny}
		}
		if pkg == "time" && name == "Time" {
			return TypeRef{Kind: KindString, Name: "datetime"}
		}
		return TypeRef{Kind: KindRef, Name: name}
	case *ast.InterfaceType:
		return TypeRef{Kind: KindAny}
	default:
		return TypeRef{Kind: KindUnknown}
	}
}

func identType(name string) TypeRef {
	switch name {
	case "string":
		return TypeRef{Kind: KindString}
	case "bool":
		return TypeRef{Kind: KindBool}
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return TypeRef{Kind: KindInt}
	case "float32", "float64":
		return TypeRef{Kind: KindNumber}
	case "byte", "rune":
		return TypeRef{Kind: KindInt}
	case "any":
		return TypeRef{Kind: KindAny}
	default:
		return TypeRef{Kind: KindRef, Name: name}
	}
}

func isPointer(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

func exprIdentName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.StarExpr:
		return exprIdentName(e.X)
	default:
		return ""
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func resolveRel(base, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Clean(filepath.Join(base, rel))
}
