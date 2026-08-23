package frontproxy

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/caddyserver/certmagic"
)

// CertMode selects where the front door's TLS certificate comes from.
type CertMode string

const (
	// CertManual uses certificate files the admin already maintains.
	CertManual CertMode = "manual"
	// CertAuto obtains and renews a certificate over ACME.
	CertAuto CertMode = "auto"
)

// TLSSettings is the certificate half of the front door. The REALITY
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
// via their own certbot -- the path for hosts that already run one.
func manualTLSConfig(s TLSSettings) (*tls.Config, error) {
	if s.CertFile == "" || s.KeyFile == "" {
		return nil, fmt.Errorf("certificate and key files are required for manual TLS")
	}
	cert, err := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading certificate: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	}, nil
}

// autoTLSConfig sets up ACME issuance and renewal via CertMagic. It mutates
// CertMagic's package templates, which is that library's intended usage and
// safe here because a process has at most one front door.
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
	// Async on purpose: a slow or failing ACME round must never block panel
	// startup. The door serves the decoy until the certificate lands.
	if err := magic.ManageAsync(ctx, []string{s.Domain}); err != nil {
		return nil, fmt.Errorf("managing certificate for %q: %w", s.Domain, err)
	}

	cfg := magic.TLSConfig()
	// TLSConfig() advertises only the ACME-ALPN protocol; without the real
	// ones alongside it, a browser negotiates nothing and the connection dies.
	cfg.NextProtos = append([]string{"h2", "http/1.1"}, cfg.NextProtos...)
	cfg.MinVersion = tls.VersionTLS12
	return cfg, nil
}
