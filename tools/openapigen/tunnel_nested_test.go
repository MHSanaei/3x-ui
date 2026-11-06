package main

import "testing"

func TestNamedConfigRemainsNested(t *testing.T) {
	schemas := []Schema{{Name: "Config", Fields: []Field{{JSONName: "role", GoName: "Role", Type: TypeRef{Kind: KindString}}}}, {Name: "Tunnel", Fields: []Field{{JSONName: "config", GoName: "Config", Type: TypeRef{Kind: KindRef, Name: "Config"}}}}}
	got := flattenEmbedded(schemas)
	if len(got[1].Fields) != 1 || got[1].Fields[0].JSONName != "config" {
		t.Fatalf("named JSON object flattened: %+v", got[1].Fields)
	}
}
