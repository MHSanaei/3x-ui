package utp

import (
	"encoding/binary"
	"net"

	"github.com/xtls/xray-core/common/dice"
)

type utp struct {
	header       byte
	extension    byte
	connectionID uint16
}

func (*utp) Size() int {
	return 4
}

func (h *utp) Serialize(b []byte) {
	binary.BigEndian.PutUint16(b, h.connectionID)
	b[2] = h.header
	b[3] = h.extension
}

type utpConn struct {
	net.PacketConn
	header *utp
}

func NewConnClient(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	conn := &utpConn{
		PacketConn: raw,
		header: &utp{
			header:       1,
			extension:    0,
			connectionID: dice.RollUint16(),
		},
	}

	return conn, nil
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	return NewConnClient(c, raw)
}

func (c *utpConn) Size() int {
	return c.header.Size()
}

func (c *utpConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return len(p) - c.header.Size(), addr, nil
}

func (c *utpConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.header.Serialize(p)

	return len(p), nil
}
