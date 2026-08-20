package pia

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServerListAdapters(t *testing.T) {
	tests := []struct {
		file, hint, schema, id, hostname string
	}{
		{"v6_valid.json", "6", "v6", "us-east", "useast401"},
		{"v7_valid.json", "7", "v7", "de-berlin", "berlin501"},
	}
	for _, test := range tests {
		raw, err := os.ReadFile(filepath.Join("testdata", "serverlist", test.file))
		if err != nil {
			t.Fatal(err)
		}
		regions, schema, err := ParseServerList(raw, test.hint)
		if err != nil {
			t.Fatalf("%s: %v", test.file, err)
		}
		if schema != test.schema || len(regions) != 1 || regions[0].ID != test.id || regions[0].WireGuard[0].Hostname != test.hostname {
			t.Fatalf("unexpected parsed result for %s: schema=%s regions=%+v", test.file, schema, regions)
		}
	}

	raw, err := os.ReadFile(filepath.Join("testdata", "serverlist", "v7_valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	regions, schema, err := ParseServerList(raw, "6")
	if err != nil || schema != "v7" || regions[0].ID != "de-berlin" {
		t.Fatalf("detected schema did not override a stale endpoint hint: schema=%q regions=%v err=%v", schema, regions, err)
	}

	legacy := []byte(`{"groups":{"wg":[]},"regions":[{"id":"legacy","name":"Legacy","country":"US","geo":false,"offline":false,"servers":{"wg":[{"ip":"198.51.100.9","cn":"legacy.example"}]}}]}`)
	regions, schema, err = ParseServerList(legacy, "")
	if err != nil || schema != "v6" || regions[0].ID != "legacy" {
		t.Fatalf("versionless v6 fallback failed: schema=%q regions=%v err=%v", schema, regions, err)
	}
}

func TestServerListRejectsMalformedFields(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "serverlist", "malformed.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ParseServerList(raw, "6"); err == nil || CodeOf(err) != CodeCatalogSchemaUnsupported {
		t.Fatalf("expected %s for malformed server list, got %s: %v", CodeCatalogSchemaUnsupported, CodeOf(err), err)
	}
}

func TestServerListRejectsUnsupportedDuplicateAndTrailingData(t *testing.T) {
	tests := []struct {
		name, raw, hint string
	}{
		{"unsupported schema", `{"version":99,"groups":{},"regions":[]}`, ""},
		{"invalid version value", `{"version":"v7beta","groups":{"wg":[]},"regions":[]}`, ""},
		{"duplicate IDs", `{"version":6,"groups":{"wg":[]},"regions":[{"id":"same","name":"One","country":"US","geo":false,"offline":false,"servers":{"wg":[{"ip":"198.51.100.1","cn":"one.example"}]}},{"id":"SAME","name":"Two","country":"US","geo":false,"offline":false,"servers":{"wg":[{"ip":"198.51.100.2","cn":"two.example"}]}}]}`, "6"},
		{"trailing JSON", `{"version":6,"groups":{},"regions":[]} {}`, "6"},
		{"wrong groups type", `{"version":6,"groups":[],"regions":[]}`, "6"},
		{"wrong field type", `{"version":6,"groups":{"wg":[]},"regions":[{"id":7,"name":"One","country":"US","geo":false,"offline":false,"servers":{"wg":[]}}]}`, "6"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := ParseServerList([]byte(test.raw), test.hint); err == nil || CodeOf(err) != CodeCatalogSchemaUnsupported {
				t.Fatalf("expected %s, got %s: %v", CodeCatalogSchemaUnsupported, CodeOf(err), err)
			}
		})
	}
}

func FuzzParseServerList(f *testing.F) {
	for _, name := range []string{"v6_valid.json", "v7_valid.json", "malformed.json"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "serverlist", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _, _ = ParseServerList(raw, "6")
	})
}
