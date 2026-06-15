package crypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"
)

// nodeCertValidity is how long the node-auth CA and the leaves it issues stay
// valid. It is deliberately long: v1 has no rotation flow, so expiry would mean
// a fleet-wide outage. Rotation is tracked as follow-up work.
const nodeCertValidity = 10 * 365 * 24 * time.Hour

// CertKeyPEM is a PEM-encoded certificate together with its private key.
type CertKeyPEM struct {
	CertPEM []byte
	KeyPEM  []byte
}

func randomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func marshalCertKey(certDER []byte, key *ecdsa.PrivateKey) (CertKeyPEM, error) {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return CertKeyPEM{}, err
	}
	return CertKeyPEM{
		CertPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
		KeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	}, nil
}

// GenerateNodeCA mints a self-signed ECDSA P-256 CA used to authenticate node
// API traffic. It can sign leaf certificates only (path length 0).
func GenerateNodeCA(commonName string) (CertKeyPEM, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return CertKeyPEM{}, err
	}
	serial, err := randomSerial()
	if err != nil {
		return CertKeyPEM{}, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(nodeCertValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return CertKeyPEM{}, err
	}
	return marshalCertKey(der, key)
}

// IssueClientCert signs a client-auth leaf (ExtKeyUsageClientAuth) with the
// given CA. The leaf authenticates the managing panel to a node.
func IssueClientCert(ca CertKeyPEM, commonName string) (CertKeyPEM, error) {
	caCert, caKey, err := LoadCAFromPEM(ca)
	if err != nil {
		return CertKeyPEM{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return CertKeyPEM{}, err
	}
	serial, err := randomSerial()
	if err != nil {
		return CertKeyPEM{}, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(nodeCertValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, key.Public(), caKey)
	if err != nil {
		return CertKeyPEM{}, err
	}
	return marshalCertKey(der, key)
}

// LoadCAFromPEM parses a CA cert+key pair into a certificate and its signer,
// ready to issue or to populate a trust pool.
func LoadCAFromPEM(ca CertKeyPEM) (*x509.Certificate, crypto.Signer, error) {
	certBlock, _ := pem.Decode(ca.CertPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, errors.New("invalid CA certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	keyBlock, _ := pem.Decode(ca.KeyPEM)
	if keyBlock == nil {
		return nil, nil, errors.New("invalid CA key PEM")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}
