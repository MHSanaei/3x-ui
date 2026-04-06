package aes128gcm_test

import (
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xtls/xray-core/common/crypto"
)

func TestAes128GcmSealInPlace(t *testing.T) {
	hashedPsk := sha256.Sum256([]byte("psk"))
	aead := crypto.NewAesGcm(hashedPsk[:16])

	text := []byte("0123456789012")
	buf := make([]byte, 8192)

	nonceSize := aead.NonceSize()
	nonce := buf[:nonceSize]
	rand.Read(nonce)
	copy(buf[nonceSize:], text)
	plaintext := buf[nonceSize : nonceSize+len(text)]

	sealed := aead.Seal(nil, nonce, plaintext, nil)

	_ = aead.Seal(plaintext[:0], nonce, plaintext, nil)

	assert.Equal(t, sealed, buf[nonceSize:nonceSize+aead.Overhead()+len(text)])
}

func encrypted(plain []byte) ([]byte, []byte) {
	hashedPsk := sha256.Sum256([]byte("psk"))
	aead := crypto.NewAesGcm(hashedPsk[:16])

	nonce := make([]byte, 12)
	rand.Read(nonce)

	return nonce, aead.Seal(nil, nonce, plain, nil)
}

func TestAes128GcmOpenInPlace(t *testing.T) {
	a, b := encrypted([]byte("0123456789012"))
	buf := make([]byte, 8192)
	copy(buf, a)
	copy(buf[len(a):], b)

	hashedPsk := sha256.Sum256([]byte("psk"))
	aead := crypto.NewAesGcm(hashedPsk[:16])

	nonceSize := aead.NonceSize()
	nonce := buf[:nonceSize]
	ciphertext := buf[nonceSize : nonceSize+len(b)]

	opened, _ := aead.Open(nil, nonce, ciphertext, nil)
	_, _ = aead.Open(ciphertext[:0], nonce, ciphertext, nil)

	assert.Equal(t, opened, ciphertext[:len(ciphertext)-aead.Overhead()])
}

func TestAes128GcmBounce(t *testing.T) {
	hashedPsk := sha256.Sum256([]byte("psk"))
	aead := crypto.NewAesGcm(hashedPsk[:16])
	buf := make([]byte, aead.NonceSize()+aead.Overhead())
	for i := 0; i < 1000; i++ {
		_, _ = rand.Read(buf)
		_, err := aead.Open(buf[aead.NonceSize():aead.NonceSize()], buf[:aead.NonceSize()], buf[aead.NonceSize():], nil)
		assert.NotEqual(t, err, nil)
	}
}
