package frontproxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/caddyserver/certmagic"
)

// CertMode selects where the reverse proxy's TLS certificate comes from.
type CertMode string

const (
	// CertManual uses certificate files the admin already maintains.
	CertManual CertMode = "manual"
	// CertAuto obtains and renews a certificate over ACME.
	CertAuto CertMode = "auto"
)

// TLSSettings is the certificate half of the reverse proxy. The REALITY
// fallback relays raw bytes, so this listener must terminate TLS itself.
type TLSSettings struct {
	Mode       CertMode
	Domain     string
	Email      string
	CertFile   string
	KeyFile    string
	StorageDir string
}

// buildTLSConfig resolves settings into a serving TLS config.
func buildTLSConfig(ctx context.Context, s TLSSettings) (*tls.Config, error) {
	if s.Mode == CertAuto {
		return autoTLSConfig(ctx, s)
	}
	return manualTLSConfig(s)
}

// manualTLSConfig loads a certificate the admin already manages, typically
// via their own certbot -- the path for hosts that already run one. Loading
// is synchronous and instant, so there is no "obtaining" phase to report;
// only the terminal obtained/failed status.
func manualTLSConfig(s TLSSettings) (*tls.Config, error) {
	if s.CertFile == "" || s.KeyFile == "" {
		err := fmt.Errorf("certificate and key files are required for manual TLS")
		setCertStatus(CertStatus{State: CertStateFailed, Error: err.Error()})
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
	if err != nil {
		err = fmt.Errorf("loading certificate: %w", err)
		setCertStatus(CertStatus{State: CertStateFailed, Error: err.Error()})
		return nil, err
	}
	status := CertStatus{State: CertStateObtained}
	if leaf, parseErr := x509.ParseCertificate(cert.Certificate[0]); parseErr == nil {
		status.NotAfter = &leaf.NotAfter
	}
	setCertStatus(status)
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	}, nil
}

// autoTLSConfig sets up ACME issuance and renewal via CertMagic. It mutates
// CertMagic's package templates, which is that library's intended usage and
// safe here because a process has at most one reverse proxy.
func autoTLSConfig(ctx context.Context, s TLSSettings) (*tls.Config, error) {
	if s.Domain == "" {
		return nil, fmt.Errorf("a domain is required for automatic certificates")
	}
	if s.StorageDir != "" {
		certmagic.Default.Storage = &certmagic.FileStorage{Path: s.StorageDir}
	}
	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = s.Email
	// Xray owns :443, so a TLS-ALPN challenge could never reach us. HTTP-01
	// on :80 is the only workable option here, same as certbot already uses.
	certmagic.DefaultACME.DisableTLSALPNChallenge = true

	magic := certmagic.NewDefault()
	magic.OnEvent = certOnEvent(magic, s.Domain)
	// Async on purpose: a slow or failing ACME round must never block panel
	// startup. The door serves the decoy until the certificate lands.
	if err := magic.ManageAsync(ctx, []string{s.Domain}); err != nil {
		err = fmt.Errorf("managing certificate for %q: %w", s.Domain, err)
		setCertStatus(CertStatus{State: CertStateFailed, Domain: s.Domain, Error: err.Error()})
		return nil, err
	}
	// ManageAsync hands off issuance/renewal to a background goroutine and
	// returns immediately, so a certificate already on disk from a previous
	// run produces no event at all this run -- seed the status from storage
	// once here so the UI shows "obtained, valid until X" right away instead
	// of staying blank until something happens to change it.
	if notAfter, err := currentCertNotAfter(magic, s.Domain); err == nil {
		setCertStatus(CertStatus{State: CertStateObtained, Domain: s.Domain, NotAfter: &notAfter})
	}

	cfg := magic.TLSConfig()
	// TLSConfig() advertises only the ACME-ALPN protocol; without the real
	// ones alongside it, a browser negotiates nothing and the connection dies.
	cfg.NextProtos = append([]string{"h2", "http/1.1"}, cfg.NextProtos...)
	cfg.MinVersion = tls.VersionTLS12
	return cfg, nil
}
