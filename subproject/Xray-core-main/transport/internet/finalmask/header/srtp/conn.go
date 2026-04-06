package srtp

import (
	"encoding/binary"
	"net"

	"github.com/xtls/xray-core/common/dice"
)

type srtp struct {
	header uint16
	number uint16
}

func (*srtp) Size() int {
	return 4
}

func (h *srtp) Serialize(b []byte) {
	h.number++
	binary.BigEndian.PutUint16(b, h.header)
	binary.BigEndian.PutUint16(b[2:], h.number)
}

type srtpConn struct {
	net.PacketConn
	header *srtp
}

func NewConnClient(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	conn := &srtpConn{
		PacketConn: raw,
		header: &srtp{
			header: 0xB5E8,
			number: dice.RollUint16(),
		},
	}

	return conn, nil
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	return NewConnClient(c, raw)
}

func (c *srtpConn) Size() int {
	return c.header.Size()
}

func (c *srtpConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return len(p) - c.header.Size(), addr, nil
}

func (c *srtpConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.header.Serialize(p)

	return len(p), nil
}
