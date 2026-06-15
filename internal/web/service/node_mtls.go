package service

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
