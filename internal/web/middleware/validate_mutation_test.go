package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The accept side of validate.go:45 — `c.ShouldBindWith(&dst, binding.JSON)` must SUCCEED
// for a well-formed JSON body and decode it into the destination struct. If the conditional
// is flipped (err != nil -> err == nil) or the bind call is dropped, a valid body would be
// rejected or the fields would come back zero-valued; both fail these assertions.
func TestBindJSONAndValidate_ValidJSONDecodesAndPasses(t *testing.T) {
	var got *sampleBody
	r := newRouter(func(c *gin.Context) {
		var ok bool
		got, ok = BindJSONAndValidate[sampleBody](c)
		if !ok {
			t.Fatalf("expected ok=true for valid JSON; got false")
		}
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/submit",
		strings.NewReader(`{"port":443,"protocol":"vless","tag":"inbound-443"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	if got == nil {
		t.Fatal("expected decoded struct; got nil")
	}
	if got.Port != 443 || got.Protocol != "vless" || got.Tag != "inbound-443" {
		t.Fatalf("decoded JSON mismatch: %+v", got)
	}
}

// The reject side of validate.go:45 — a malformed JSON body must be caught by the bind
// conditional, returning (nil,false) with a parse-error Message and NO validator Issues.
// If the conditional is flipped so malformed input bypasses the bind check, control falls
// through to validate.Struct on a zero-valued struct, which would instead emit validator
// Issues (e.g. rule="required"/"gte"). Asserting empty Issues + non-empty Message pins the
// distinct parse-failure path that line 45 owns.
func TestBindJSONAndValidate_MalformedJSONRejectedWithoutValidatorIssues(t *testing.T) {
	r := newRouter(func(c *gin.Context) {
		if _, ok := BindJSONAndValidate[sampleBody](c); ok {
			t.Fatal("expected ok=false on malformed JSON; got true")
		}
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/submit",
		strings.NewReader(`{"port":}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	msg := decodeMsg(t, rec.Body.String())
	if msg.Success {
		t.Fatal("expected Success=false on malformed JSON")
	}
	payload, err := payloadFromObj(msg.Obj)
	if err != nil {
		t.Fatalf("payload extraction: %v", err)
	}
	if len(payload.Issues) != 0 {
		t.Fatalf("expected empty Issues for a JSON parse error (not validator output); got %+v", payload.Issues)
	}
	if payload.Message == "" {
		t.Fatal("expected non-empty Message describing the JSON parse error")
	}
}

// BindJSONAndValidateInto shares the same line-45-style bind conditional (line 57). Cover its
// accept side: a valid JSON body must bind onto the caller-supplied destination and pass,
// overwriting any pre-populated field. A flipped/dropped bind check leaves the destination
// untouched (or returns false), which these assertions catch.
func TestBindJSONAndValidateInto_ValidJSONBindsOntoDestination(t *testing.T) {
	dst := &sampleBody{Tag: "preset"}
	r := newRouter(func(c *gin.Context) {
		if !BindJSONAndValidateInto(c, dst) {
			t.Fatal("expected ok=true for valid JSON; got false")
		}
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/submit",
		strings.NewReader(`{"port":8443,"protocol":"trojan","tag":"inbound-8443"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	if dst.Port != 8443 || dst.Protocol != "trojan" {
		t.Fatalf("expected JSON to bind onto destination; got %+v", dst)
	}
	if dst.Tag != "inbound-8443" {
		t.Fatalf("expected payload Tag to overwrite preset; got %q", dst.Tag)
	}
}
