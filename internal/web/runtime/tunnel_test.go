package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/wattunnel"
)

func TestTunnelRemoteUsesBasePathAndDoesNotReflectSecrets(t *testing.T) {
	secret := "never-reflect-this-secret"
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/private/panel/api/tunnels/sync" {
			t.Errorf("wrong base path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(secret))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	remote := NewRemote(&model.Node{Scheme: "https", Address: u.Hostname(), Port: port, BasePath: "/private/", TlsVerifyMode: "verify", ApiToken: "api-key"}, nil)
	remote.clientOnce.Do(func() { remote.client = srv.Client() })
	_, err := remote.ApplyTunnel(context.Background(), wattunnel.Spec{Secret: secret})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("remote error leaked request: %v", err)
	}
	remote.node.TlsVerifyMode = "skip"
	if _, err = remote.ApplyTunnel(context.Background(), wattunnel.Spec{}); err == nil || !strings.Contains(err.Error(), "verified HTTPS") {
		t.Fatalf("insecure node accepted: %v", err)
	}
}

func TestTunnelOldNodeCapabilitiesAreUnsupported(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	remote := NewRemote(&model.Node{Scheme: "https", Address: u.Hostname(), Port: port, BasePath: "/", TlsVerifyMode: "verify"}, nil)
	remote.clientOnce.Do(func() { remote.client = srv.Client() })
	caps, err := remote.TunnelCapabilities(context.Background())
	if err == nil || caps.Supported || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("old node falsely advertised support: %+v, %v", caps, err)
	}
}

func TestNodeRedirectCannotDowngradeTunnelSecrets(t *testing.T) {
	first, _ := http.NewRequest(http.MethodPost, "https://node.example:443/panel/api/tunnels/sync", nil)
	downgrade, _ := http.NewRequest(http.MethodPost, "http://node.example:443/panel/api/tunnels/sync", nil)
	if err := NoCrossHostRedirect(downgrade, []*http.Request{first}); err == nil {
		t.Fatal("HTTPS node client followed a plaintext redirect")
	}
}
