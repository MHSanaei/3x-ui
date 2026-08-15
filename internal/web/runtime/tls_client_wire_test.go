package runtime

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type wireObservation struct {
	pin        string
	remoteAddr string
}

func startLeafRecordingServer(t *testing.T) (*httptest.Server, *x509.CertPool, func() []wireObservation) {
	t.Helper()
	var mu sync.Mutex
	var seen []wireObservation
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observation := wireObservation{remoteAddr: r.RemoteAddr}
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			sum := sha256.Sum256(r.TLS.PeerCertificates[0].Raw)
			observation.pin = hex.EncodeToString(sum[:])
		}
		mu.Lock()
		seen = append(seen, observation)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return srv, pool, func() []wireObservation {
		mu.Lock()
		defer mu.Unlock()
		result := make([]wireObservation, len(seen))
		copy(result, seen)
		return result
	}
}

func pinOf(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	sum := sha256.Sum256(cert.Certificate[0])
	return hex.EncodeToString(sum[:])
}

func rotatingClientForTest(t *testing.T, roots *x509.CertPool) *http.Client {
	t.Helper()
	build := func() (idleClosingRoundTripper, error) {
		cert, err := getMasterClientCert()
		if err != nil {
			return nil, err
		}
		return &http.Transport{
			MaxIdleConns:        64,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     60 * time.Second,
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      roots,
				MinVersion:   tls.VersionTLS12,
			},
		}, nil
	}
	transport, err := newCredentialRotatingTransport(build)
	if err != nil {
		t.Fatalf("newCredentialRotatingTransport: %v", err)
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func doWireRequest(t *testing.T, client *http.Client, url string) {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want=%d", response.StatusCode, http.StatusOK)
	}
}

func TestCredentialRotationPresentsNewLeafOnNextConnection(t *testing.T) {
	server, roots, observations := startLeafRecordingServer(t)
	oldCert := masterCertForTest(t)
	newCert := masterCertForTest(t)
	oldPin := pinOf(t, oldCert)
	newPin := pinOf(t, newCert)
	if oldPin == newPin {
		t.Fatal("test fixture produced identical leaves")
	}
	var providerMu sync.Mutex
	current := oldCert
	SetMasterClientCertProvider(func() (tls.Certificate, error) {
		providerMu.Lock()
		defer providerMu.Unlock()
		return current, nil
	})
	t.Cleanup(func() { SetMasterClientCertProvider(nil) })
	client := rotatingClientForTest(t, roots)
	doWireRequest(t, client, server.URL)
	doWireRequest(t, client, server.URL)
	baseline := observations()
	if len(baseline) != 2 || baseline[0].pin != oldPin || baseline[1].pin != oldPin {
		t.Fatalf("baseline=%v", baseline)
	}
	if baseline[0].remoteAddr != baseline[1].remoteAddr {
		t.Fatalf("baseline connections differ: %v", baseline)
	}
	providerMu.Lock()
	current = newCert
	providerMu.Unlock()
	InvalidateMasterClientConnections()
	doWireRequest(t, client, server.URL)
	after := observations()
	if len(after) != 3 || after[2].pin != newPin {
		t.Fatalf("rotation observations=%v want new leaf=%s", after, newPin)
	}
	if after[2].remoteAddr == baseline[1].remoteAddr {
		t.Fatalf("rotated request reused stale connection %s", after[2].remoteAddr)
	}
}

func TestCredentialRotationControlKeepsOldLeafWithoutInvalidation(t *testing.T) {
	server, roots, observations := startLeafRecordingServer(t)
	oldCert := masterCertForTest(t)
	newCert := masterCertForTest(t)
	oldPin := pinOf(t, oldCert)
	var providerMu sync.Mutex
	current := oldCert
	SetMasterClientCertProvider(func() (tls.Certificate, error) {
		providerMu.Lock()
		defer providerMu.Unlock()
		return current, nil
	})
	t.Cleanup(func() { SetMasterClientCertProvider(nil) })
	client := rotatingClientForTest(t, roots)
	doWireRequest(t, client, server.URL)
	providerMu.Lock()
	current = newCert
	providerMu.Unlock()
	doWireRequest(t, client, server.URL)
	got := observations()
	if len(got) != 2 || got[1].pin != oldPin {
		t.Fatalf("control observations=%v want stale leaf=%s", got, oldPin)
	}
	if got[0].remoteAddr != got[1].remoteAddr {
		t.Fatalf("control did not reuse connection: %v", got)
	}
}
