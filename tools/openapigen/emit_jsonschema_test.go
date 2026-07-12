package main

import "testing"

func TestIntegerSchemaFormats(t *testing.T) {
	tests := []struct {
		name       string
		goType     string
		wantFormat string
	}{
		{name: "int", goType: "int"},
		{name: "int32", goType: "int32"},
		{name: "int64", goType: "int64", wantFormat: "int64"},
		{name: "uint64", goType: "uint64", wantFormat: "int64"},
	}

	gen := &schemaGen{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := gen.typeSchema(identType(tt.goType))
			if got := schema["type"]; got != "integer" {
				t.Fatalf("type = %v, want integer", got)
			}
			got, _ := schema["format"].(string)
			if got != tt.wantFormat {
				t.Fatalf("format = %q, want %q", got, tt.wantFormat)
			}
		})
	}
}
