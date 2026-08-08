package service

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

const (
	settingNodeMtlsCaCert     = "nodeMtlsCaCertPem"
	settingNodeMtlsCaKey      = "nodeMtlsCaKeyPem"
	settingNodeMtlsClientCert = "nodeMtlsClientCertPem"
	settingNodeMtlsClientKey  = "nodeMtlsClientKeyPem"
	settingNodeMtlsClientCA   = "nodeMtlsClientCAPem"
)

// EnsureNodeMtlsCA returns this panel's node-auth CA, minting and persisting it
// on first use and reusing the stored pair thereafter. The CA private key never
// leaves the panel.
func (s *SettingService) EnsureNodeMtlsCA() (crypto.CertKeyPEM, error) {
	certPem, err := s.getString(settingNodeMtlsCaCert)
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	keyPem, err := s.getString(settingNodeMtlsCaKey)
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	if certPem != "" && keyPem != "" {
		return crypto.CertKeyPEM{CertPEM: []byte(certPem), KeyPEM: []byte(keyPem)}, nil
	}
	// Fail closed on a half-present pair: regenerating here would silently rotate
	// the CA and break trust on nodes that already hold the old cert. Only mint
	// when neither half exists (first use).
	if certPem != "" || keyPem != "" {
		return crypto.CertKeyPEM{}, common.NewError("node mTLS CA is incomplete: one of cert/key is missing; refusing to regenerate")
	}
	ca, err := crypto.GenerateNodeCA("3x-ui node mTLS CA")
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	if err := s.saveSetting(settingNodeMtlsCaCert, string(ca.CertPEM)); err != nil {
		return crypto.CertKeyPEM{}, err
	}
	if err := s.saveSetting(settingNodeMtlsCaKey, string(ca.KeyPEM)); err != nil {
		return crypto.CertKeyPEM{}, err
	}
	return ca, nil
}

// EnsureMasterClientCert returns the client certificate this panel presents when
// calling its nodes over mTLS, issuing it from the node CA on first use and
// reusing the stored pair thereafter.
func (s *SettingService) EnsureMasterClientCert() (crypto.CertKeyPEM, error) {
	certPem, err := s.getString(settingNodeMtlsClientCert)
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	keyPem, err := s.getString(settingNodeMtlsClientKey)
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	if certPem != "" && keyPem != "" {
		return crypto.CertKeyPEM{CertPEM: []byte(certPem), KeyPEM: []byte(keyPem)}, nil
	}
	// Half a stored pair signals corrupted settings; reissuing would rotate the
	// master client credential (and indirectly the CA). Only mint on first use.
	if certPem != "" || keyPem != "" {
		return crypto.CertKeyPEM{}, common.NewError("master client cert is incomplete: one of cert/key is missing; refusing to reissue")
	}
	ca, err := s.EnsureNodeMtlsCA()
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	client, err := crypto.IssueClientCert(ca, "3x-ui master")
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	if err := s.saveSetting(settingNodeMtlsClientCert, string(client.CertPEM)); err != nil {
		return crypto.CertKeyPEM{}, err
	}
	if err := s.saveSetting(settingNodeMtlsClientKey, string(client.KeyPEM)); err != nil {
		return crypto.CertKeyPEM{}, err
	}
	return client, nil
}

// NodeMtlsClientCAPool builds the trust pool used as the panel listener's
// ClientCAs for incoming node-API client certificates. It returns (nil, nil)
// when no trust CA is configured, so mTLS stays off and the listener behaves
// exactly as before.
func (s *SettingService) NodeMtlsClientCAPool() (*x509.CertPool, error) {
	caPem, err := s.getString(settingNodeMtlsClientCA)
	if err != nil {
		return nil, err
	}
	if caPem == "" {
		return nil, nil
	}
	certs, err := parseCertificateBundlePEM([]byte(caPem))
	if err != nil {
		return nil, common.NewError("nodeMtlsClientCAPem is not a valid certificate bundle: ", err)
	}
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return pool, nil
}

// parseCertificateBundlePEM validates every non-whitespace byte of a PEM
// certificate bundle and returns the certificates it contains.
// x509.CertPool.AppendCertsFromPEM reports success once it has parsed a single
// certificate, so a bundle whose later entries are damaged or truncated is
// accepted with those entries silently missing from the pool.
func parseCertificateBundlePEM(bundle []byte) ([]*x509.Certificate, error) {
	rest := bytes.TrimSpace(bundle)
	if len(rest) == 0 {
		return nil, common.NewError("certificate bundle is empty")
	}
	certs := make([]*x509.Certificate, 0, 1)
	for len(rest) > 0 {
		block, next := pem.Decode(rest)
		if block == nil {
			return nil, common.NewError("certificate bundle contains malformed or non-PEM data")
		}
		if block.Type != "CERTIFICATE" {
			return nil, common.NewError("certificate bundle contains a non-certificate PEM block")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
		rest = bytes.TrimSpace(next)
	}
	return certs, nil
}
