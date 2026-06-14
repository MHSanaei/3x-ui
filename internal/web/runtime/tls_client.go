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
	"github.com/mhsanaei/3x-ui/v3/internal/util/netproxy"
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

func HTTPClientForNode(n *model.Node, proxyURL string) (*http.Client, error) {
	mode := n.TlsVerifyMode
	if mode == "" {
		mode = "verify"
	}
	if proxyURL != "" {
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
