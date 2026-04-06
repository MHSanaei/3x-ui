package dtls

import (
	"net"

	"github.com/xtls/xray-core/common/dice"
)

type dtls struct {
	epoch    uint16
	length   uint16
	sequence uint32
}

func (*dtls) Size() int {
	return 1 + 2 + 2 + 6 + 2
}

func (h *dtls) Serialize(b []byte) {
	b[0] = 23
	b[1] = 254
	b[2] = 253
	b[3] = byte(h.epoch >> 8)
	b[4] = byte(h.epoch)
	b[5] = 0
	b[6] = 0
	b[7] = byte(h.sequence >> 24)
	b[8] = byte(h.sequence >> 16)
	b[9] = byte(h.sequence >> 8)
	b[10] = byte(h.sequence)
	h.sequence++
	b[11] = byte(h.length >> 8)
	b[12] = byte(h.length)
	h.length += 17
	if h.length > 100 {
		h.length -= 50
	}
}

type dtlsConn struct {
	net.PacketConn
	header *dtls
}

func NewConnClient(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	conn := &dtlsConn{
		PacketConn: raw,
		header: &dtls{
			epoch:    dice.RollUint16(),
			sequence: 0,
			length:   17,
		},
	}

	return conn, nil
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	return NewConnClient(c, raw)
}

func (c *dtlsConn) Size() int {
	return c.header.Size()
}

func (c *dtlsConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return len(p) - c.header.Size(), addr, nil
}

func (c *dtlsConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.header.Serialize(p)

	return len(p), nil
}
