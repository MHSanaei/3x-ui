package aes128gcm

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"net"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/transport/internet/finalmask"
)

type aes128gcmConn struct {
	net.PacketConn
	aead cipher.AEAD
}

func NewConnClient(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	hashedPsk := sha256.Sum256([]byte(c.Password))

	conn := &aes128gcmConn{
		PacketConn: raw,
		aead:       crypto.NewAesGcm(hashedPsk[:16]),
	}

	return conn, nil
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	return NewConnClient(c, raw)
}

func (c *aes128gcmConn) Size() int {
	return c.aead.NonceSize()
}

func (c *aes128gcmConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if len(p) < c.aead.NonceSize()+c.aead.Overhead() {
		return 0, addr, errors.New("aead short lenth")
	}

	nonceSize := c.aead.NonceSize()
	nonce := p[:nonceSize]
	ciphertext := p[nonceSize:]
	_, err = c.aead.Open(ciphertext[:0], nonce, ciphertext, nil)
	if err != nil {
		return 0, addr, errors.New("aead open").Base(err)
	}

	return len(p) - c.aead.NonceSize() - c.aead.Overhead(), addr, nil
}

func (c *aes128gcmConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if c.aead.Overhead()+len(p) > finalmask.UDPSize {
		return 0, errors.New("aead short write")
	}

	nonceSize := c.aead.NonceSize()
	nonce := p[:nonceSize]
	common.Must2(rand.Read(nonce))
	plaintext := p[nonceSize:]
	_ = c.aead.Seal(plaintext[:0], nonce, plaintext, nil)

	return len(p) + c.aead.Overhead(), nil
}
