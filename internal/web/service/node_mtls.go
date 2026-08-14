package service

import (
	"crypto/tls"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
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

// ReloadMasterMtlsClient validates the master credential currently stored by
// the panel and drops cached mTLS connection pools. This makes an intentional
// out-of-process credential rotation take effect without restarting x-ui (and
// therefore without stopping the xray child process in the same service).
func (s *NodeService) ReloadMasterMtlsClient() error {
	stored, err := (&SettingService{}).LoadMasterClientCert()
	if err != nil {
		return err
	}
	if _, err := tls.X509KeyPair(stored.CertPEM, stored.KeyPEM); err != nil {
		return err
	}
	runtime.InvalidateMasterClientConnections()
	return nil
}

// SetNodeMtlsTrustCA stores the CA certificate bundle trusted for incoming
// node-API clients. An empty value clears it; changes apply after restart.
func (s *NodeService) SetNodeMtlsTrustCA(caPem string) error {
	caPem = strings.TrimSpace(caPem)
	if caPem != "" {
		if _, err := parseCertificateBundlePEM([]byte(caPem)); err != nil {
			return common.NewError("invalid trust CA certificate bundle: ", err)
		}
	}
	return (&SettingService{}).setString(settingNodeMtlsClientCA, caPem)
}
