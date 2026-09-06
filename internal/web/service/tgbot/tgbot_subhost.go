package tgbot

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"strings"
)

// The OS hostname is never reachable from the internet, so falling back to it hands the
// customer an unresolvable URL; the configured certificate names the real domain.
func domainFromCertificate(certFile string) string {
	if certFile == "" {
		return ""
	}
	raw, err := os.ReadFile(certFile)
	if err != nil {
		return ""
	}
	for block, rest := pem.Decode(raw); block != nil; block, rest = pem.Decode(rest) {
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if name := preferredCertName(cert); name != "" {
			return name
		}
	}
	return ""
}

// A wildcard cannot be used as a host, but it still reveals the zone, so it is
// kept as a last resort behind any concrete name the certificate carries.
func preferredCertName(cert *x509.Certificate) string {
	wildcard := ""
	for _, name := range cert.DNSNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, "*.") {
			if wildcard == "" {
				wildcard = strings.TrimPrefix(name, "*.")
			}
			continue
		}
		return name
	}
	if wildcard != "" {
		return wildcard
	}
	if cn := strings.TrimSpace(cert.Subject.CommonName); cn != "" && !strings.HasPrefix(cn, "*.") {
		return cn
	}
	return ""
}
