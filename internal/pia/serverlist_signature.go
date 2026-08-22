package pia

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"unicode"
)

func VerifySignedServerList(raw, publicKeyPEM []byte) ([]byte, error) {
	jsonBody, signature, err := splitSignedServerList(raw)
	if err != nil {
		return nil, WrapError(CodeCatalogSignatureInvalid, "The PIA region list signature is missing or invalid.", err)
	}
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, NewError(CodeCatalogSignatureInvalid, "The built-in region-list public key is invalid.")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, WrapError(CodeCatalogSignatureInvalid, "The built-in region-list public key is invalid.", err)
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, NewError(CodeCatalogSignatureInvalid, "The region-list public key is not RSA.")
	}
	digest := sha256.Sum256(jsonBody)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, WrapError(CodeCatalogSignatureInvalid, "The PIA region list signature does not match its content.", err)
	}
	return jsonBody, nil
}

func splitSignedServerList(raw []byte) ([]byte, []byte, error) {
	if len(raw) == 0 || raw[0] != '{' {
		return nil, nil, fmt.Errorf("response does not start with a JSON object")
	}
	end := bytes.LastIndexByte(raw, '}')
	if end < 0 || end == len(raw)-1 {
		return nil, nil, fmt.Errorf("appended signature is absent")
	}
	jsonBody := append([]byte(nil), raw[:end+1]...)
	encoded := bytes.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, raw[end+1:])
	if len(encoded) == 0 {
		return nil, nil, fmt.Errorf("appended signature is empty")
	}
	signature := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	n, err := base64.StdEncoding.Decode(signature, encoded)
	if err != nil {
		return nil, nil, fmt.Errorf("decode signature: %w", err)
	}
	return jsonBody, signature[:n], nil
}
