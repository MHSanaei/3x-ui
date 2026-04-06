package wechat

import (
	"encoding/binary"
	"net"

	"github.com/xtls/xray-core/common/dice"
)

type wechat struct {
	sn uint32
}

func (*wechat) Size() int {
	return 13
}

func (h *wechat) Serialize(b []byte) {
	h.sn++
	b[0] = 0xa1
	b[1] = 0x08
	binary.BigEndian.PutUint32(b[2:], h.sn)
	b[6] = 0x00
	b[7] = 0x10
	b[8] = 0x11
	b[9] = 0x18
	b[10] = 0x30
	b[11] = 0x22
	b[12] = 0x30
}

type wechatConn struct {
	net.PacketConn
	header *wechat
}

func NewConnClient(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	conn := &wechatConn{
		PacketConn: raw,
		header: &wechat{
			sn: uint32(dice.RollUint16()),
		},
	}

	return conn, nil
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	return NewConnClient(c, raw)
}

func (c *wechatConn) Size() int {
	return c.header.Size()
}

func (c *wechatConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return len(p) - c.header.Size(), addr, nil
}

func (c *wechatConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.header.Serialize(p)

	return len(p), nil
}
