package original_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xtls/xray-core/transport/internet/finalmask/mkcp/original"
)

func TestSimpleSealInPlace(t *testing.T) {
	aead := original.NewSimple()

	text := []byte("0123456789012")
	buf := make([]byte, 8192)

	copy(buf[aead.Overhead():], text)
	plaintext := buf[aead.Overhead() : aead.Overhead()+len(text)]

	sealed := aead.Seal(nil, nil, plaintext, nil)

	_ = aead.Seal(buf[:0], nil, plaintext, nil)

	assert.Equal(t, sealed, buf[:aead.Overhead()+len(text)])
}

func TestOriginalBounce(t *testing.T) {
	aead := original.NewSimple()
	buf := make([]byte, aead.NonceSize()+aead.Overhead())
	for i := 0; i < 1000; i++ {
		_, _ = rand.Read(buf)
		_, err := aead.Open(buf[:0], nil, buf, nil)
		assert.NotEqual(t, err, nil)
	}
}
