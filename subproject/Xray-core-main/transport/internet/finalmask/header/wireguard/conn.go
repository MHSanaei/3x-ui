package wireguard

import (
	"net"
)

type wireguare struct{}

func (*wireguare) Size() int {
	return 4
}

func (h *wireguare) Serialize(b []byte) {
	b[0] = 0x04
	b[1] = 0x00
	b[2] = 0x00
	b[3] = 0x00
}

type wireguareConn struct {
	net.PacketConn
	header *wireguare
}

func NewConnClient(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	conn := &wireguareConn{
		PacketConn: raw,
		header:     &wireguare{},
	}

	return conn, nil
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	return NewConnClient(c, raw)
}

func (c *wireguareConn) Size() int {
	return c.header.Size()
}

func (c *wireguareConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return len(p) - c.header.Size(), addr, nil
}

func (c *wireguareConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.header.Serialize(p)

	return len(p), nil
}
