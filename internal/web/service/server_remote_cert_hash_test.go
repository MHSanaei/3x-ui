package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

func TestGetRemoteCertHashGuardsPrivateTargets(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "https://")
	sum := sha256.Sum256(srv.Certificate().Raw)
	want := hex.EncodeToString(sum[:])

	t.Run("loopback refused without opt-in", func(t *testing.T) {
		hashes, err := (&ServerService{}).GetRemoteCertHash(target, false)
		if !errors.Is(err, netsafe.ErrPrivateAddressBlocked) {
			t.Fatalf("GetRemoteCertHash(%s) = %v, %v; want ErrPrivateAddressBlocked", target, hashes, err)
		}
	})

	t.Run("loopback read with opt-in", func(t *testing.T) {
		hashes, err := (&ServerService{}).GetRemoteCertHash(target, true)
		if err != nil {
			t.Fatalf("GetRemoteCertHash(%s, allowPrivate): %v", target, err)
		}
		if len(hashes) != 1 || hashes[0] != want {
			t.Fatalf("hashes = %v, want [%s]", hashes, want)
		}
	})
}
