package controller

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestPanelRestartScopeAndDelayedHook(t *testing.T) {
	engine, auth := newAPIAuthTestEngine(t)
	NewSettingController(engine.Group("/panel/api", auth.checkAPIAuth, auth.enforceTokenScope))
	restarts := make(chan time.Time, 1)
	global.SetRestartHook(func() { restarts <- time.Now() })
	t.Cleanup(func() { global.SetRestartHook(func() {}) })
	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeNodeAdmin, model.ApiScopeNodeSync, model.ApiScopeMonitor} {
		if err := database.GetDB().Create(&model.ApiToken{Name: scope, Token: crypto.HashTokenSHA256(scope), Enabled: true, Scope: scope}).Error; err != nil {
			t.Fatal(err)
		}
		for _, mtls := range []bool{false, true} {
			t.Run(scope+map[bool]string{false: "/bearer", true: "/mtls"}[mtls], func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/panel/api/setting/restartPanel", nil)
				req.Header.Set("Authorization", "Bearer "+scope)
				if mtls {
					req.TLS = &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{}}}}
				}
				w := httptest.NewRecorder()
				start := time.Now()
				engine.ServeHTTP(w, req)
				allowed := scope == model.ApiScopeNodeAdmin || (scope == model.ApiScopeAdmin && !mtls)
				if !allowed {
					want := `{"msg":"this API token is not permitted to access this endpoint","success":false}`
					if w.Code != http.StatusForbidden || w.Body.String() != want {
						t.Fatalf("denied = %d %s", w.Code, w.Body.String())
					}
					return
				}
				var result struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.Success || w.Code != http.StatusOK {
					t.Fatalf("accepted = %d %s: %v", w.Code, w.Body.String(), err)
				}
				select {
				case <-restarts:
					t.Fatal("panel restarted before response was delivered")
				default:
				}
				select {
				case at := <-restarts:
					if at.Sub(start) < 3*time.Second {
						t.Fatalf("restart delay = %s, want >=3s", at.Sub(start))
					}
				case <-time.After(6 * time.Second):
					t.Fatal("accepted request did not restart panel")
				}
			})
		}
	}
}

func writeLoadableKeyPair(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotAfter: time.Now().Add(time.Hour)}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certFile = filepath.Join(t.TempDir(), "panel.crt")
	keyFile = filepath.Join(t.TempDir(), "panel.key")
	for file, block := range map[string]*pem.Block{
		certFile: {Type: "CERTIFICATE", Bytes: certDER},
		keyFile:  {Type: "PRIVATE KEY", Bytes: keyDER},
	} {
		if err := os.WriteFile(file, pem.EncodeToMemory(block), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return certFile, keyFile
}

// The settings page turns this answer into its post-restart URL. Guessing from stored
// settings sent a browser behind a TLS proxy to http:// and ignored XUI_PORT.
func TestPanelRestartReportsServingAndNextListener(t *testing.T) {
	engine, auth := newAPIAuthTestEngine(t)
	NewSettingController(engine.Group("/panel/api", auth.checkAPIAuth, auth.enforceTokenScope))
	if err := database.GetDB().Create(&model.ApiToken{Name: "admin", Token: crypto.HashTokenSHA256("admin"), Enabled: true, Scope: model.ApiScopeAdmin}).Error; err != nil {
		t.Fatal(err)
	}
	certFile, keyFile := writeLoadableKeyPair(t)
	settings := service.SettingService{}

	cases := []struct {
		name         string
		servingTLS   bool
		cert, key    string
		portOverride string
		wantNextPort int
		wantNextTLS  bool
	}{
		{name: "plain listener, nothing configured", wantNextPort: 3000},
		{name: "plain listener, key pair configured", cert: certFile, key: keyFile, wantNextPort: 3000, wantNextTLS: true},
		{name: "a certificate without its key stays plain", cert: certFile, wantNextPort: 3000},
		{name: "XUI_PORT outranks the stored port", portOverride: "8443", wantNextPort: 8443},
		{name: "TLS listener, key pair removed", servingTLS: true, wantNextPort: 3000},
	}
	restarts := make(chan struct{}, len(cases))
	global.SetRestartHook(func() { restarts <- struct{}{} })
	var requests int
	t.Cleanup(func() {
		defer global.SetRestartHook(func() {})
		// Drain this test's scheduled restarts before another test installs its hook.
		timeout := time.NewTimer(6 * time.Second)
		defer timeout.Stop()
		for range requests {
			select {
			case <-restarts:
			case <-timeout.C:
				t.Error("restart callbacks from listener requests did not finish")
				return
			}
		}
	})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XUI_PORT", tc.portOverride)
			if err := settings.SetPort(3000); err != nil {
				t.Fatal(err)
			}
			if err := settings.SetCertFile(tc.cert); err != nil {
				t.Fatal(err)
			}
			if err := settings.SetKeyFile(tc.key); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewUnstartedServer(engine)
			if tc.servingTLS {
				server.StartTLS()
			} else {
				server.Start()
			}
			t.Cleanup(server.Close)
			u, _ := url.Parse(server.URL)
			servingPort, _ := strconv.Atoi(u.Port())

			req, _ := http.NewRequest(http.MethodPost, server.URL+"/panel/api/setting/restartPanel", nil)
			req.Header.Set("Authorization", "Bearer admin")
			requests++
			res, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			var answer struct {
				Success bool                `json:"success"`
				Obj     entity.PanelRestart `json:"obj"`
			}
			if err := json.NewDecoder(res.Body).Decode(&answer); err != nil || !answer.Success {
				t.Fatalf("status %d, success=%v, err=%v", res.StatusCode, answer.Success, err)
			}
			want := entity.PanelRestart{Port: servingPort, TLS: tc.servingTLS, NextPort: tc.wantNextPort, NextTLS: tc.wantNextTLS}
			if answer.Obj != want {
				t.Fatalf("restart answer = %+v, want %+v", answer.Obj, want)
			}
		})
	}
}
