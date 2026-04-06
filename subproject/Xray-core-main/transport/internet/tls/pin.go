package tls

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
)

// []byte must be ASN.1 DER content
func GenerateCertHash[T *x509.Certificate | []byte](cert T) []byte {
	var out [32]byte
	switch v := any(cert).(type) {
	case *x509.Certificate:
		out = sha256.Sum256(v.Raw)
	case []byte:
		out = sha256.Sum256(v)
	}
	return out[:]
}

func GenerateCertHashHex[T *x509.Certificate | []byte](cert T) string {
	var out [32]byte
	switch v := any(cert).(type) {
	case *x509.Certificate:
		out = sha256.Sum256(v.Raw)
	case []byte:
		out = sha256.Sum256(v)
	}
	return hex.EncodeToString(out[:])
}
