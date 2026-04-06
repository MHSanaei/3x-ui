package original

import (
	"crypto/cipher"
	"encoding/binary"
	"hash/fnv"
	"net"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
)

type simple struct{}

func NewSimple() *simple {
	return &simple{}
}

func (*simple) NonceSize() int {
	return 0
}

func (*simple) Overhead() int {
	return 6
}

func (a *simple) Seal(dst, nonce, plain, extra []byte) []byte {
	dst = append(dst, 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint16(dst[4:], uint16(len(plain)))
	dst = append(dst, plain...)

	fnvHash := fnv.New32a()
	common.Must2(fnvHash.Write(dst[4:]))
	fnvHash.Sum(dst[:0])

	dstLen := len(dst)
	xtra := 4 - dstLen%4
	if xtra != 4 {
		dst = append(dst, make([]byte, xtra)...)
	}
	xorfwd(dst)
	if xtra != 4 {
		dst = dst[:dstLen]
	}
	return dst
}

func (a *simple) Open(dst, nonce, cipherText, extra []byte) ([]byte, error) {
	dst = append(dst, cipherText...)
	dstLen := len(dst)
	xtra := 4 - dstLen%4
	if xtra != 4 {
		dst = append(dst, make([]byte, xtra)...)
	}
	xorbkd(dst)
	if xtra != 4 {
		dst = dst[:dstLen]
	}

	fnvHash := fnv.New32a()
	common.Must2(fnvHash.Write(dst[4:]))
	if binary.BigEndian.Uint32(dst[:4]) != fnvHash.Sum32() {
		return nil, errors.New("invalid auth")
	}

	length := binary.BigEndian.Uint16(dst[4:6])
	if len(dst)-6 != int(length) {
		return nil, errors.New("invalid auth")
	}

	return dst[6:], nil
}

type simpleConn struct {
	net.PacketConn
	aead cipher.AEAD
}

func NewConnClient(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	conn := &simpleConn{
		PacketConn: raw,
		aead:       &simple{},
	}

	return conn, nil
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	return NewConnClient(c, raw)
}

func (c *simpleConn) Size() int {
	return c.aead.Overhead()
}

func (c *simpleConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	_, err = c.aead.Open(p[:0], nil, p, nil)
	if err != nil {
		return 0, addr, errors.New("aead open").Base(err)
	}

	return len(p) - c.aead.Overhead(), addr, nil
}

func (c *simpleConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	_ = c.aead.Seal(p[:0], nil, p[c.aead.Overhead():], nil)

	return len(p), nil
}
