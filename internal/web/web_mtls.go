package web

import (
	"crypto/tls"
	"crypto/x509"
)

// applyNodeMtls configures the panel listener to request and verify client
// certificates against pool. It uses VerifyClientCertIfGiven so browsers (which
// present no client cert) keep working; a presented cert that fails to verify
// aborts the handshake. With a nil pool the config is left untouched, so the
// no-mTLS listener is byte-identical to before.
func applyNodeMtls(cfg *tls.Config, pool *x509.CertPool) {
	if pool == nil {
		return
	}
	cfg.ClientAuth = tls.VerifyClientCertIfGiven
	cfg.ClientCAs = pool
}
