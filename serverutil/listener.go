package serverutil

import (
	"crypto/tls"
	"net"

	"github.com/mhsanaei/3x-ui/v2/web/network"
)

// TLSWrapResult captures listener wrapping outcome.
type TLSWrapResult struct {
	Listener net.Listener
	HTTPS    bool
	CertErr  error
}

// WrapListenerWithOptionalTLS wraps listener with auto HTTPS + TLS when cert/key are valid.
// If cert loading fails, it returns the original listener and the certificate error.
func WrapListenerWithOptionalTLS(listener net.Listener, certFile, keyFile string) TLSWrapResult {
	if certFile == "" || keyFile == "" {
		return TLSWrapResult{Listener: listener, HTTPS: false}
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return TLSWrapResult{Listener: listener, HTTPS: false, CertErr: err}
	}

	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	wrapped := network.NewAutoHttpsListener(listener)
	wrapped = tls.NewListener(wrapped, config)
	return TLSWrapResult{Listener: wrapped, HTTPS: true}
}
