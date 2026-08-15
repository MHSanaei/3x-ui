package tgbot

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateNumericInput(t *testing.T) {
	tests := []struct {
		name       string
		value, key int
		want       int
	}{
		{name: "append digit", value: 12, key: 3, want: 123},
		{name: "append zero", value: 12, key: 0, want: 120},
		{name: "backspace", value: 123, key: -1, want: 12},
		{name: "backspace zero", value: 0, key: -1, want: 0},
		{name: "clear", value: 123, key: -2, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateNumericInput(tt.value, tt.key); got != tt.want {
				t.Fatalf("updateNumericInput(%d, %d) = %d, want %d", tt.value, tt.key, got, tt.want)
			}
		})
	}
}

func TestNumericInputTransitionIsUsedByEveryKeypad(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read tgbot package: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || name == "numeric_input.go" || filepath.Ext(strings.TrimSuffix(name, "_test.go")) != ".go" {
			continue
		}
		parsed, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switchStmt, ok := node.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			hasClear, hasBackspace, hasDefault := false, false, false
			for _, stmt := range switchStmt.Body.List {
				clause := stmt.(*ast.CaseClause)
				if clause.List == nil {
					hasDefault = true
				}
				for _, expr := range clause.List {
					hasClear = hasClear || numericKeyLiteral(expr, "2")
					hasBackspace = hasBackspace || numericKeyLiteral(expr, "1")
				}
			}
			if hasClear && hasBackspace && hasDefault {
				position := fset.Position(switchStmt.Pos())
				t.Errorf("open-coded numeric keypad transition at %s; use updateNumericInput", position)
			}
			return true
		})
	}
}

func numericKeyLiteral(expr ast.Expr, magnitude string) bool {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op != token.SUB {
		return false
	}
	literal, ok := unary.X.(*ast.BasicLit)
	return ok && literal.Kind == token.INT && literal.Value == magnitude
}
