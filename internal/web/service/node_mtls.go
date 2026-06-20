package service

import (
	"crypto/x509"
	"encoding/pem"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// NodeMtlsCaCert returns the PEM of this panel's node-auth CA certificate (the
// public half) to copy into a node's mTLS trust setting, minting the CA and the
// master client cert on first call so the panel is ready to present a client
// certificate to mtls nodes.
func (s *NodeService) NodeMtlsCaCert() (string, error) {
	settings := SettingService{}
	ca, err := settings.EnsureNodeMtlsCA()
	if err != nil {
		return "", err
	}
	if _, err := settings.EnsureMasterClientCert(); err != nil {
		return "", err
	}
	return string(ca.CertPEM), nil
}

// SetNodeMtlsTrustCA stores the CA certificate this panel trusts for incoming
// node-API client certificates. An empty value clears it (mTLS off). A
// non-empty value must be a PEM certificate (fail closed). Takes effect on the
// next panel restart, when the listener's ClientCAs is rebuilt.
func (s *NodeService) SetNodeMtlsTrustCA(caPem string) error {
	caPem = strings.TrimSpace(caPem)
	if caPem != "" {
		block, _ := pem.Decode([]byte(caPem))
		if block == nil || block.Type != "CERTIFICATE" {
			return common.NewError("trust CA must be a PEM-encoded certificate")
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return common.NewError("invalid trust CA certificate: " + err.Error())
		}
	}
	return (&SettingService{}).setString(settingNodeMtlsClientCA, caPem)
}
