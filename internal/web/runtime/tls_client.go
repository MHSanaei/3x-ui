package runtime

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netproxy"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

// MasterClientCertProvider supplies the master client certificate this panel
// presents to nodes in mtls mode. It is injected by the web layer so the
// runtime package need not import service.
type MasterClientCertProvider func() (tls.Certificate, error)

var (
	masterClientCertMu sync.RWMutex
	masterClientCert   MasterClientCertProvider
	masterCertEpoch    atomic.Uint64
)

// SetMasterClientCertProvider installs the provider used to obtain the master
// client certificate for mtls nodes. Passing nil disables it.
func SetMasterClientCertProvider(p MasterClientCertProvider) {
	masterClientCertMu.Lock()
	defer masterClientCertMu.Unlock()
	masterClientCert = p
}

func getMasterClientCert() (tls.Certificate, error) {
	masterClientCertMu.RLock()
	p := masterClientCert
	masterClientCertMu.RUnlock()
	if p == nil {
		return tls.Certificate{}, common.NewError("mtls: master client certificate provider not configured")
	}
	return p()
}

// InvalidateMasterClientConnections advances the client-credential generation.
// Every cached mTLS transport observes the generation before its next request,
// replaces its TLS transport, and closes the old idle pool. Requests already
// in flight are not interrupted; no request that starts after invalidation can
// reuse a connection authenticated with the previous leaf.
func InvalidateMasterClientConnections() {
	masterCertEpoch.Add(1)
}

// ReloadMasterClientConnections validates that the currently configured
// provider can load the master credential, then invalidates every cached mTLS
// transport. Operators that rotate the credential outside the process (for
// example by restoring settings) can call this without restarting the panel.
func ReloadMasterClientConnections() error {
	if _, err := getMasterClientCert(); err != nil {
		return err
	}
	InvalidateMasterClientConnections()
	return nil
}

type idleClosingRoundTripper interface {
	http.RoundTripper
	CloseIdleConnections()
}

type credentialRotatingTransport struct {
	mu         sync.Mutex
	generation uint64
	current    idleClosingRoundTripper
	build      func() (idleClosingRoundTripper, error)
}

func buildStableCredentialTransport(build func() (idleClosingRoundTripper, error)) (idleClosingRoundTripper, uint64, error) {
	for {
		before := masterCertEpoch.Load()
		current, err := build()
		if err != nil {
			return nil, 0, err
		}
		after := masterCertEpoch.Load()
		if before == after {
			return current, after, nil
		}
		current.CloseIdleConnections()
	}
}

func newCredentialRotatingTransport(build func() (idleClosingRoundTripper, error)) (*credentialRotatingTransport, error) {
	current, generation, err := buildStableCredentialTransport(build)
	if err != nil {
		return nil, err
	}
	return &credentialRotatingTransport{
		generation: generation,
		current:    current,
		build:      build,
	}, nil
}

func (t *credentialRotatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	if masterCertEpoch.Load() != t.generation {
		next, generation, err := buildStableCredentialTransport(t.build)
		if err != nil {
			t.mu.Unlock()
			return nil, err
		}
		previous := t.current
		t.current = next
		t.generation = generation
		previous.CloseIdleConnections()
	}
	current := t.current
	t.mu.Unlock()
	return current.RoundTrip(req)
}

func (t *credentialRotatingTransport) CloseIdleConnections() {
	t.mu.Lock()
	current := t.current
	t.mu.Unlock()
	current.CloseIdleConnections()
}

// defaultNodeHTTPClient reaches nodes trusting the system CA store ("verify"
// mode or plain http); shared so connections pool across nodes.
var defaultNodeHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
		DialContext:         netsafe.SSRFGuardedDialContext,
	},
}

func HTTPClientForNode(n *model.Node, proxyURL string) (*http.Client, error) {
	mode := n.TlsVerifyMode
	if mode == "" {
		mode = "verify"
	}
	if proxyURL != "" {
		if mode == "mtls" && n.Scheme != "http" {
			timeout := remoteHTTPTimeout
			build := func() (idleClosingRoundTripper, error) {
				client, err := netproxy.NewHTTPClient(proxyURL, remoteHTTPTimeout)
				if err != nil {
					return nil, err
				}
				transport, ok := client.Transport.(*http.Transport)
				if !ok {
					return nil, common.NewError("mtls proxy client transport does not support credential rotation")
				}
				tlsCfg, err := tlsConfigForNode(n)
				if err != nil {
					return nil, err
				}
				transport.TLSClientConfig = tlsCfg
				return transport, nil
			}
			transport, err := newCredentialRotatingTransport(build)
			if err != nil {
				return nil, err
			}
			return &http.Client{Transport: transport, Timeout: timeout}, nil
		}
		client, err := netproxy.NewHTTPClient(proxyURL, remoteHTTPTimeout)
		if err != nil {
			return nil, err
		}
		if mode == "verify" || n.Scheme == "http" {
			return client, nil
		}
		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			return client, nil
		}
		tlsCfg, err := tlsConfigForNode(n)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig = tlsCfg
		return client, nil
	}
	if mode == "verify" || n.Scheme == "http" {
		return defaultNodeHTTPClient, nil
	}
	if mode == "mtls" {
		build := func() (idleClosingRoundTripper, error) {
			tlsCfg, err := tlsConfigForNode(n)
			if err != nil {
				return nil, err
			}
			return &http.Transport{
				MaxIdleConns:        64,
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     60 * time.Second,
				DialContext:         netsafe.SSRFGuardedDialContext,
				TLSClientConfig:     tlsCfg,
			}, nil
		}
		transport, err := newCredentialRotatingTransport(build)
		if err != nil {
			return nil, err
		}
		return &http.Client{Transport: transport}, nil
	}
	tlsCfg, err := tlsConfigForNode(n)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        64,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     60 * time.Second,
			DialContext:         netsafe.SSRFGuardedDialContext,
			TLSClientConfig:     tlsCfg,
		},
	}, nil
}

func tlsConfigForNode(n *model.Node) (*tls.Config, error) {
	if n.TlsVerifyMode == "mtls" {
		// Present the master client cert; verify the node's server cert against
		// the system roots (no InsecureSkipVerify). mtls authenticates the
		// caller — it does not change how the node's server identity is checked.
		cert, err := getMasterClientCert()
		if err != nil {
			return nil, err
		}
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}, nil
	}
	tlsCfg := &tls.Config{InsecureSkipVerify: true} // lgtm[go/disabled-certificate-check]
	if n.TlsVerifyMode == "pin" {
		want, err := DecodeCertPin(n.PinnedCertSha256)
		if err != nil {
			return nil, err
		}
		tlsCfg.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return common.NewError("node presented no certificate")
			}
			sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
			if subtle.ConstantTimeCompare(sum[:], want) != 1 {
				return common.NewError("node certificate does not match pinned SHA-256")
			}
			return nil
		}
	}
	return tlsCfg, nil
}

// DecodeCertPin decodes a SHA-256 cert pin given as base64 (Xray's
// pinnedPeerCertSha256 form) or hex with optional colons into 32 raw bytes.
func DecodeCertPin(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, common.NewError("certificate pin is empty")
	}
	if b, err := hex.DecodeString(strings.ReplaceAll(s, ":", "")); err == nil && len(b) == sha256.Size {
		return b, nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == sha256.Size {
			return b, nil
		}
	}
	return nil, common.NewError("certificate pin must be a SHA-256 hash (base64 or hex)")
}
