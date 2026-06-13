package runtime

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

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

// HTTPClientForNode returns the node's HTTP client honoring its TLS verify mode
// (verify→system CA, skip→no check, pin→leaf SHA-256). Used by both the probe
// and every Remote op so they can't disagree on a self-signed node (#5264).
func HTTPClientForNode(n *model.Node) (*http.Client, error) {
	mode := n.TlsVerifyMode
	if mode == "" {
		mode = "verify"
	}
	if mode == "verify" || n.Scheme == "http" {
		return defaultNodeHTTPClient, nil
	}
	tlsCfg := &tls.Config{InsecureSkipVerify: true} // lgtm[go/disabled-certificate-check]
	if mode == "pin" {
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
