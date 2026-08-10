package service

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"strings"
	"sync"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

var masterClientCredentialMu sync.Mutex

const (
	settingNodeMtlsCaCert     = "nodeMtlsCaCertPem"
	settingNodeMtlsCaKey      = "nodeMtlsCaKeyPem"
	settingNodeMtlsClientCert = "nodeMtlsClientCertPem"
	settingNodeMtlsClientKey  = "nodeMtlsClientKeyPem"
	settingNodeMtlsClientPin  = "nodeMtlsClientCertSha256"
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

func clientCertSHA256FromPEM(certPEM []byte) (string, error) {
	block, rest := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return "", common.NewError("client certificate is not valid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:]), nil
}

// EnsureMasterClientCert returns the client certificate this panel presents when
// calling its nodes over mTLS, issuing it from the node CA on first use and
// reusing the stored pair thereafter.
func (s *SettingService) EnsureMasterClientCert() (crypto.CertKeyPEM, error) {
	masterClientCredentialMu.Lock()
	defer masterClientCredentialMu.Unlock()

	certPem, err := s.getString(settingNodeMtlsClientCert)
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	keyPem, err := s.getString(settingNodeMtlsClientKey)
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	storedPin, err := s.getString(settingNodeMtlsClientPin)
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	storedPin = strings.ToLower(strings.TrimSpace(storedPin))
	if certPem != "" && keyPem != "" {
		actualPin, err := clientCertSHA256FromPEM([]byte(certPem))
		if err != nil {
			return crypto.CertKeyPEM{}, err
		}
		if storedPin != "" && storedPin != actualPin {
			return crypto.CertKeyPEM{}, common.NewError("stored master client certificate does not match nodeMtlsClientCertSha256; refusing to rotate")
		}
		if storedPin == "" {
			if err := s.saveSetting(settingNodeMtlsClientPin, actualPin); err != nil {
				return crypto.CertKeyPEM{}, err
			}
		}
		return crypto.CertKeyPEM{CertPEM: []byte(certPem), KeyPEM: []byte(keyPem)}, nil
	}
	// Half a stored pair signals corrupted settings; reissuing would rotate the
	// master client credential (and indirectly the CA). Only mint on first use.
	if certPem != "" || keyPem != "" {
		return crypto.CertKeyPEM{}, common.NewError("master client cert is incomplete: one of cert/key is missing; refusing to reissue")
	}
	// A surviving pin proves that this panel previously had a master client
	// credential. Minting a replacement here would silently strand every node
	// already enforcing the old leaf identity after a partial/old DR restore.
	if storedPin != "" {
		return crypto.CertKeyPEM{}, common.NewError("master client credential is missing while nodeMtlsClientCertSha256 is present; refusing to reissue")
	}
	ca, err := s.EnsureNodeMtlsCA()
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	client, err := crypto.IssueClientCert(ca, "3x-ui master")
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	pin, err := clientCertSHA256FromPEM(client.CertPEM)
	if err != nil {
		return crypto.CertKeyPEM{}, err
	}
	if err := saveMasterClientCredential(client, pin); err != nil {
		return crypto.CertKeyPEM{}, err
	}
	return client, nil
}

func saveMasterClientCredential(client crypto.CertKeyPEM, pin string) error {
	values := map[string]string{
		settingNodeMtlsClientCert: string(client.CertPEM),
		settingNodeMtlsClientKey:  string(client.KeyPEM),
		settingNodeMtlsClientPin:  pin,
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		for key, value := range values {
			result := tx.Model(&model.Setting{}).Where("key = ?", key).Update("value", value)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				if err := tx.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
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
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caPem)) {
		return nil, common.NewError("nodeMtlsClientCAPem is not a valid certificate")
	}
	return pool, nil
}
