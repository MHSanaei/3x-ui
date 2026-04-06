package cert

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/task"
)

func TestGenerate(t *testing.T) {
	err := generate(nil, true, true, "ca")
	if err != nil {
		t.Fatal(err)
	}
}

func generate(domainNames []string, isCA bool, jsonOutput bool, fileOutput string) error {
	commonName := "Xray Root CA"
	organization := "Xray Inc"

	expire := time.Hour * 3

	var opts []Option
	if isCA {
		opts = append(opts, Authority(isCA))
		opts = append(opts, KeyUsage(x509.KeyUsageCertSign|x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature))
	}

	opts = append(opts, NotAfter(time.Now().Add(expire)))
	opts = append(opts, CommonName(commonName))
	if len(domainNames) > 0 {
		opts = append(opts, DNSNames(domainNames...))
	}
	opts = append(opts, Organization(organization))

	cert, err := Generate(nil, opts...)
	if err != nil {
		return errors.New("failed to generate TLS certificate").Base(err)
	}

	if jsonOutput {
		printJSON(cert)
	}

	if len(fileOutput) > 0 {
		if err := printFile(cert, fileOutput); err != nil {
			return err
		}
	}

	return nil
}

type jsonCert struct {
	Certificate []string `json:"certificate"`
	Key         []string `json:"key"`
}

func printJSON(certificate *Certificate) {
	certPEM, keyPEM := certificate.ToPEM()
	jCert := &jsonCert{
		Certificate: strings.Split(strings.TrimSpace(string(certPEM)), "\n"),
		Key:         strings.Split(strings.TrimSpace(string(keyPEM)), "\n"),
	}
	content, err := json.MarshalIndent(jCert, "", "  ")
	common.Must(err)
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func printFile(certificate *Certificate, name string) error {
	certPEM, keyPEM := certificate.ToPEM()
	return task.Run(context.Background(), func() error {
		return writeFile(certPEM, name+".crt")
	}, func() error {
		return writeFile(keyPEM, name+".key")
	})
}

func writeFile(content []byte, name string) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()

	return common.Error2(f.Write(content))
}
