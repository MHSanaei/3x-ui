package xray

import (
	"strings"
	"testing"
)

func TestRemoveUserGuardsNilHandlerClient(t *testing.T) {
	err := (&XrayAPI{}).RemoveUser("in-443-tcp", "user@example.com")
	if err == nil {
		t.Fatal("RemoveUser with an uninitialized HandlerServiceClient must return an error")
	}
}

func TestGetRequiredUserString_Present(t *testing.T) {
	user := map[string]any{"email": "alice@example.com"}
	got, err := getRequiredUserString(user, "email")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alice@example.com" {
		t.Fatalf("got %q, want %q", got, "alice@example.com")
	}
}

func TestGetRequiredUserString_Missing(t *testing.T) {
	user := map[string]any{}
	if _, err := getRequiredUserString(user, "email"); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestGetRequiredUserString_NilValue(t *testing.T) {
	user := map[string]any{"email": nil}
	if _, err := getRequiredUserString(user, "email"); err == nil {
		t.Fatal("expected error for nil value")
	}
}

func TestGetRequiredUserString_WrongType(t *testing.T) {
	user := map[string]any{"email": 42}
	_, err := getRequiredUserString(user, "email")
	if err == nil {
		t.Fatal("expected error for non-string value")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Fatalf("expected %q in error, got: %v", "invalid type", err)
	}
}

func TestGetOptionalUserString_Present(t *testing.T) {
	user := map[string]any{"flow": "xtls-rprx-vision"}
	got, err := getOptionalUserString(user, "flow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "xtls-rprx-vision" {
		t.Fatalf("got %q, want %q", got, "xtls-rprx-vision")
	}
}

func TestGetOptionalUserString_MissingReturnsEmptyNoError(t *testing.T) {
	user := map[string]any{}
	got, err := getOptionalUserString(user, "flow")
	if err != nil {
		t.Fatalf("unexpected error for missing optional field: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestGetOptionalUserString_NilReturnsEmptyNoError(t *testing.T) {
	user := map[string]any{"flow": nil}
	got, err := getOptionalUserString(user, "flow")
	if err != nil {
		t.Fatalf("unexpected error for nil optional field: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestGetOptionalUserString_WrongTypeErrors(t *testing.T) {
	user := map[string]any{"flow": []string{"a", "b"}}
	if _, err := getOptionalUserString(user, "flow"); err == nil {
		t.Fatal("expected error for non-string optional value")
	}
}
