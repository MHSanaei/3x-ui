package pia

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestNilErrorHelpers(t *testing.T) {
	if code := CodeOf(nil); code != "" {
		t.Fatalf("CodeOf(nil)=%q, want empty", code)
	}
	var typed *Error
	var err error = typed
	if code := CodeOf(err); code != CodeNetworkUnavailable {
		t.Fatalf("CodeOf(nil *Error)=%q, want %q", code, CodeNetworkUnavailable)
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		t.Fatalf("errors.Unwrap(nil *Error)=%v, want nil", unwrapped)
	}
}

func TestValidWGKeyRequiresBase64Encoded32Bytes(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if !validWGKey(valid) {
		t.Fatalf("valid WireGuard key rejected: %q", valid)
	}
	for _, invalid := range []string{
		"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!",
		base64.StdEncoding.EncodeToString(make([]byte, 31)),
		base64.StdEncoding.EncodeToString(make([]byte, 33)),
	} {
		if validWGKey(invalid) {
			t.Fatalf("invalid WireGuard key accepted: %q", invalid)
		}
	}
}
