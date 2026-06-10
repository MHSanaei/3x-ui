package controller

import (
	"encoding/json"
	"testing"
)

func TestWithServerBasePath(t *testing.T) {
	spec := []byte(`{"openapi":"3.0.3","info":{"title":"x"},"servers":[{"url":"/","description":"old"}],"paths":{"/p":{"get":{"summary":"s"}}}}`)

	out, err := withServerBasePath(spec, "/test/")
	if err != nil {
		t.Fatalf("withServerBasePath: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	servers, ok := doc["servers"].([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("servers = %v, want one entry", doc["servers"])
	}
	srv, _ := servers[0].(map[string]any)
	if srv["url"] != "/test" {
		t.Errorf("server url = %v, want /test (trailing slash trimmed)", srv["url"])
	}

	if doc["openapi"] != "3.0.3" {
		t.Errorf("openapi field not preserved: %v", doc["openapi"])
	}
	if _, ok := doc["paths"].(map[string]any)["/p"]; !ok {
		t.Errorf("paths content not preserved verbatim")
	}
}

func TestWithServerBasePathInvalidJSON(t *testing.T) {
	if _, err := withServerBasePath([]byte("not json"), "/test/"); err == nil {
		t.Errorf("expected error on invalid spec, got nil")
	}
}
