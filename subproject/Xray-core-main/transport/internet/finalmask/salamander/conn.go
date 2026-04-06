package salamander

import (
	"net"

	"github.com/xtls/xray-core/common/errors"
)

type salamanderConn struct {
	net.PacketConn
	obfs *SalamanderObfuscator
}

func NewConnClient(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	ob, err := NewSalamanderObfuscator([]byte(c.Password))
	if err != nil {
		return nil, errors.New("salamander err").Base(err)
	}

	conn := &salamanderConn{
		PacketConn: raw,
		obfs:       ob,
	}

	return conn, nil
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	return NewConnClient(c, raw)
}

func (c *salamanderConn) Size() int {
	return smSaltLen
}

func (c *salamanderConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	c.obfs.Deobfuscate(p, p[smSaltLen:])

	return len(p) - smSaltLen, addr, nil
}

func (c *salamanderConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.obfs.Obfuscate(p[smSaltLen:], p)

	return len(p), nil
}
