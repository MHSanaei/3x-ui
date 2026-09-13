package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/chacha20poly1305"
)

// vdfzfoff public key from Omegaplexx/hpwnr 3745cb96e2551e003cb217ab7705b4d67f8ac006, src/keys.rs.
// Salted Crypt5 with separator V passed Android 4.3.0 and Windows 4.1.2 import/update probes.
const happPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA9+umWSxp8coKnMONnI4u
NvtPErJZt8VNgNb2XS+RrCMc9AFWZQH01ILr3Py/mviuqFgNLMEcPs3k6+ZPh6Sa
OCXHmjQicGPJAw6Co6GQwO/b4vspHgOM4HSvX5r6SY1EKIHUSLIyRV28DfwJKdFv
x2EKqypewlrAo4AV76uI/9U+1t40yHcVCj/OtFxsq+mMM6qySieTsA1q6C5raBrJ
u3l/RWMxFYvDInYDs1IaTFGDFwSdFDqhNU19gPGloT/GApy+U32R6AGSxJymS2nh
e6pm/M9bvsH0o0Oc1kyXsBpVN04n/a9gVVUoqODzrUyXDx7/jAzNJD43PWtblcz0
ZNBKN50wvpSD5UuAQydwMT7xWJIpPaZqTUj/sg8hIm57XGlUxRCge17nB0Ff7sKO
JAgaXVdbfqDdzx+PhSaZY9xfcAh/sHfE6hKaCQ9kIn5cjbx9bcYqZWnpuSOzSFg+
CgMSqvG6rV6d+96dNMHuE0tRIUJ83xrLcm9hZJmJ6WDm6hteZbnb1k3eQF9c+XCF
wSEvsWiXyduQmkVNJaCRXwy8tSaZp9JftALhRHMvd7Eq6ctAkvn7w0upynsAtLeL
N8xZ5q1gcRgboydr588D3m8KF7mVuX/XRp2AG7hzyYdkQov9bfEfXIaBVlwHMKhy
uPTxeM4Les6fvaHMSWJ+8EUCAwEAAQ==
-----END PUBLIC KEY-----`

const (
	happCrypt5Marker         = "vdfzfoff"
	happPublicKeyFingerprint = "22319c7b13647897bf5fd4f827ba92bf3946d738007a0054ccd931c31f221768"
	// Bound application work before encoding; this is not a promised Happ client limit.
	happMaxSourceBytes = 8192
)

func encryptHappLink(source string) (string, error) {
	key, err := checkedHappPublicKey([]byte(happPublicKeyPEM))
	if err != nil {
		return "", err
	}
	return encryptHappSource(source, key)
}

func encryptHappSource(source string, key *rsa.PublicKey) (string, error) {
	if !validHappRSAKey(key) {
		return "", ErrHappLinkUnavailable
	}
	if len(source) > happMaxSourceBytes {
		return "", ErrHappSourceTooLong
	}
	if len(source) == 0 || !utf8.ValidString(source) || strings.IndexFunc(source, unicode.IsControl) >= 0 {
		return "", ErrHappLinkUnavailable
	}
	parsed, err := url.Parse(source)
	if err != nil || !parsed.IsAbs() || parsed.Opaque != "" || parsed.Hostname() == "" ||
		parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", ErrHappLinkUnavailable
	}

	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const alnum = letters + "0123456789"
	sessionKey := make([]byte, 32)
	if _, err := rand.Read(sessionKey); err != nil {
		return "", fmt.Errorf("happ key randomness: %w", err)
	}
	nonce, err := randomHappCharacters(12, alnum)
	if err != nil {
		return "", err
	}
	tag, err := randomHappCharacters(2, letters)
	if err != nil {
		return "", err
	}
	salt, err := randomHappCharacters(8, alnum)
	if err != nil {
		return "", err
	}
	wrappedKey := make([]byte, 32)
	for i := range wrappedKey {
		wrappedKey[i] = sessionKey[i] ^ salt[i%8]
	}
	rsaPlain := swapHappPairs([]byte(base64.StdEncoding.EncodeToString(wrappedKey)))
	//nolint:staticcheck // Happ Crypt5 requires PKCS#1 v1.5 key wrapping; OAEP changes the wire format.
	rsaCipher, err := rsa.EncryptPKCS1v15(rand.Reader, key, rsaPlain)
	if err != nil {
		return "", fmt.Errorf("happ RSA wrapping: %w", err)
	}
	aead, err := chacha20poly1305.New(sessionKey)
	if err != nil {
		return "", fmt.Errorf("happ AEAD initialization: %w", err)
	}
	// Parsing validates the URL but must not normalize its UTF-8, escapes, or query bytes.
	plain := swapHappPairs([]byte(base64.StdEncoding.EncodeToString([]byte(source))))
	cipherB64 := base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plain, nil))
	body := string(nonce) + string(tag) + string(salt) + strconv.Itoa(len(cipherB64)) +
		"V" + cipherB64 + base64.StdEncoding.EncodeToString(rsaCipher)
	frame := []byte(happCrypt5Marker[:4] + body + happCrypt5Marker[4:])
	for i := 0; i+3 < len(frame); i += 4 {
		frame[i], frame[i+2] = frame[i+2], frame[i]
		frame[i+1], frame[i+3] = frame[i+3], frame[i+1]
	}
	return "happ://crypt5/" + string(frame), nil
}

func checkedHappPublicKey(pemData []byte) (*rsa.PublicKey, error) {
	trimmed := bytes.TrimSpace(pemData)
	block, rest := pem.Decode(trimmed)
	if !bytes.HasPrefix(trimmed, []byte("-----BEGIN PUBLIC KEY-----")) || block == nil ||
		block.Type != "PUBLIC KEY" || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ErrHappLinkUnavailable
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, ErrHappLinkUnavailable
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok || !validHappRSAKey(key) {
		return nil, ErrHappLinkUnavailable
	}
	spki, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, ErrHappLinkUnavailable
	}
	fingerprint := sha256.Sum256(spki)
	// The marker selects the client's private key, so accepting any RSA public key would be incorrect.
	if hex.EncodeToString(fingerprint[:]) != happPublicKeyFingerprint {
		return nil, ErrHappLinkUnavailable
	}
	return key, nil
}

func validHappRSAKey(key *rsa.PublicKey) bool {
	return key != nil && key.N != nil && key.N.Sign() > 0 && key.N.BitLen() == 4096 && key.N.Bit(0) == 1 && key.E == 65537
}

func randomHappCharacters(length int, alphabet string) ([]byte, error) {
	result := make([]byte, length)
	limit := big.NewInt(int64(len(alphabet)))
	for i := range result {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return nil, fmt.Errorf("happ character randomness: %w", err)
		}
		result[i] = alphabet[index.Int64()]
	}
	return result, nil
}

func swapHappPairs(data []byte) []byte {
	for i := 0; i+1 < len(data); i += 2 {
		data[i], data[i+1] = data[i+1], data[i]
	}
	return data
}
