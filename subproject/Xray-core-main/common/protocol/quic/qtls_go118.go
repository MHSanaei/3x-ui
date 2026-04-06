package quic

import (
	"crypto"
	"crypto/cipher"
	_ "crypto/tls"
	_ "unsafe"
)

type CipherSuiteTLS13 struct {
	ID     uint16
	KeyLen int
	AEAD   func(key, fixedNonce []byte) cipher.AEAD
	Hash   crypto.Hash
}

//go:linkname AEADAESGCMTLS13 crypto/tls.aeadAESGCMTLS13
func AEADAESGCMTLS13(key, nonceMask []byte) cipher.AEAD
