package wattunnel

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed LICENSE
var License []byte

//go:embed release-key.pem
var releaseKey []byte

func VerifyArtifacts(binary, manifest, signature string) error {
	sums, err := os.ReadFile(manifest)
	if err != nil {
		return errors.New("WatTunnel signed checksum manifest missing")
	}
	sig, err := os.ReadFile(signature)
	if err != nil {
		return errors.New("WatTunnel release signature missing")
	}
	block, _ := pem.Decode(releaseKey)
	if block == nil {
		return errors.New("invalid embedded release key")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	pub, ok := key.(*rsa.PublicKey)
	if !ok {
		return errors.New("release signing key is not RSA")
	}
	digest := sha256.Sum256(sums)
	if err = rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		return errors.New("WatTunnel release signature invalid")
	}
	expected := ""
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filepath.Base(binary) {
			if expected != "" {
				return errors.New("duplicate release checksum")
			}
			expected = fields[0]
		}
	}
	if len(expected) != 64 {
		return errors.New("WatTunnel binary absent from signed manifest")
	}
	f, err := os.Open(binary)
	if err != nil {
		return errors.New("WatTunnel binary missing")
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("WatTunnel binary checksum mismatch")
	}
	return nil
}

func VerifyBinary(binary string) error {
	dir := filepath.Dir(binary)
	if err := VerifyArtifacts(binary, filepath.Join(dir, "wattunnel-SHA256SUMS"), filepath.Join(dir, "wattunnel-SHA256SUMS.sig")); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-version").Output()
	if err != nil {
		return errors.New("cannot read WatTunnel version")
	}
	if strings.TrimSpace(string(out)) != "wattun "+Version {
		return errors.New("unsupported WatTunnel version; expected " + Version)
	}
	return nil
}
