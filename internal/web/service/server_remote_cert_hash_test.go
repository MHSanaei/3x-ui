package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

// A node reports its inbounds' certificate paths; they name files on the node, so the
// master must not read its own file of that name when the admin asks for a pin.
func TestGetCertHashIgnoresPathsANodeReports(t *testing.T) {
	setupSettingTestDB(t)
	certFile := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(certFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	nodeID := 1
	stream := `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":` + strconv.Quote(certFile) + `}]}}`
	if err := database.GetDB().Create(&model.Inbound{Tag: "node-in", Port: 44312, Protocol: model.VLESS, NodeID: &nodeID, StreamSettings: stream}).Error; err != nil {
		t.Fatalf("seed node inbound: %v", err)
	}
	_, err := (&ServerService{}).GetCertHash(certFile, "")
	if err == nil || !strings.Contains(err.Error(), "not referenced") {
		t.Fatalf("GetCertHash(node-reported path) = %v, want it treated as unknown", err)
	}
}
