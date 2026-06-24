package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A real XTLS .dgst sidecar (Xray-linux-64.zip.dgst, v26.3.27): lines are
// "ALGO= <hex>", and the algorithm label is "SHA2-256", not "SHA256".
const sampleXrayDgst = `# Hash Values

MD5= ee4e2ff74948a9b464624b1cabc44409
SHA1= b55b06e74e89083b9cedfdecf0d68b579cd2af72
SHA2-256= 23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae
SHA2-512= e8bc40a0687cac184bbe4b5c1f047e69064ccedc489fb25e208889ae287bbf8736dff16b108d68fc00dc33edc8bb53502e47a9698a277f4f51b67b83d899e518
`

const wantSHA = "23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae"

func TestParseXrayDigestSHA256(t *testing.T) {
	got, err := parseXrayDigestSHA256([]byte(sampleXrayDgst))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != wantSHA {
		t.Fatalf("sha = %q, want %q", got, wantSHA)
	}
}

func TestParseXrayDigestSHA256_Errors(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
	}{
		{"no-sha256-line", "MD5= abc\nSHA1= def\n"},
		{"malformed-short", "SHA2-256= deadbeef\n"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseXrayDigestSHA256([]byte(tc.in)); err == nil {
				t.Fatalf("%s: expected an error", tc.name)
			}
		})
	}
}

func TestFetchXrayDigestSHA256(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleXrayDgst))
	}))
	defer srv.Close()

	got, err := (&ServerService{}).fetchXrayDigestSHA256(srv.Client(), srv.URL+"/Xray-linux-64.zip.dgst")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got != wantSHA {
		t.Fatalf("sha = %q, want %q", got, wantSHA)
	}
}

func TestFetchXrayDigestSHA256_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := (&ServerService{}).fetchXrayDigestSHA256(srv.Client(), srv.URL+"/missing.dgst"); err == nil {
		t.Fatal("expected an error on HTTP 404")
	}
}
