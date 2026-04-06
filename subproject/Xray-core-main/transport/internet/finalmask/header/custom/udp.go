package custom

import (
	"bytes"
	"net"

	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/common/errors"
)

type udpCustomClient struct {
	client []*UDPItem
	server []*UDPItem
	merged []byte
}

func (h *udpCustomClient) Serialize(b []byte) {
	index := 0
	for _, item := range h.client {
		if item.Rand > 0 {
			crypto.RandBytesBetween(h.merged[index:index+int(item.Rand)], byte(item.RandMin), byte(item.RandMax))
			index += int(item.Rand)
		} else {
			index += len(item.Packet)
		}
	}
	copy(b, h.merged)
}

func (h *udpCustomClient) Match(b []byte) bool {
	if len(b) < len(h.merged) {
		return false
	}

	data := b
	match := true

	for _, item := range h.server {
		length := max(int(item.Rand), len(item.Packet))

		if len(item.Packet) > 0 && !bytes.Equal(item.Packet, data[:length]) {
			match = false
			break
		}

		data = data[length:]
	}

	return match
}

type udpCustomClientConn struct {
	net.PacketConn
	header *udpCustomClient
}

func NewConnClientUDP(c *UDPConfig, raw net.PacketConn) (net.PacketConn, error) {
	conn := &udpCustomClientConn{
		PacketConn: raw,
		header: &udpCustomClient{
			client: c.Client,
			server: c.Server,
		},
	}

	index := 0
	for _, item := range conn.header.client {
		if item.Rand > 0 {
			conn.header.merged = append(conn.header.merged, make([]byte, item.Rand)...)
			index += int(item.Rand)
		} else {
			conn.header.merged = append(conn.header.merged, item.Packet...)
			index += len(item.Packet)
		}
	}

	return conn, nil
}

func (c *udpCustomClientConn) Size() int {
	return len(c.header.merged)
}

func (c *udpCustomClientConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if !c.header.Match(p) {
		return 0, addr, errors.New("header mismatch")
	}

	return len(p) - len(c.header.merged), addr, nil
}

func (c *udpCustomClientConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.header.Serialize(p)

	return len(p), nil
}

type udpCustomServer struct {
	client []*UDPItem
	server []*UDPItem
	merged []byte
}

func (h *udpCustomServer) Serialize(b []byte) {
	index := 0
	for _, item := range h.server {
		if item.Rand > 0 {
			crypto.RandBytesBetween(h.merged[index:index+int(item.Rand)], byte(item.RandMin), byte(item.RandMax))
			index += int(item.Rand)
		} else {
			index += len(item.Packet)
		}
	}
	copy(b, h.merged)
}

func (h *udpCustomServer) Match(b []byte) bool {
	if len(b) < len(h.merged) {
		return false
	}

	data := b
	match := true

	for _, item := range h.client {
		length := max(int(item.Rand), len(item.Packet))

		if len(item.Packet) > 0 && !bytes.Equal(item.Packet, data[:length]) {
			match = false
			break
		}

		data = data[length:]
	}

	return match
}

type udpCustomServerConn struct {
	net.PacketConn
	header *udpCustomServer
}

func NewConnServerUDP(c *UDPConfig, raw net.PacketConn) (net.PacketConn, error) {
	conn := &udpCustomServerConn{
		PacketConn: raw,
		header: &udpCustomServer{
			client: c.Client,
			server: c.Server,
		},
	}

	index := 0
	for _, item := range conn.header.server {
		if item.Rand > 0 {
			conn.header.merged = append(conn.header.merged, make([]byte, item.Rand)...)
			index += int(item.Rand)
		} else {
			conn.header.merged = append(conn.header.merged, item.Packet...)
			index += len(item.Packet)
		}
	}

	return conn, nil
}

func (c *udpCustomServerConn) Size() int {
	return len(c.header.merged)
}

func (c *udpCustomServerConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if !c.header.Match(p) {
		return 0, addr, errors.New("header mismatch")
	}

	return len(p) - len(c.header.merged), addr, nil
}

func (c *udpCustomServerConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.header.Serialize(p)

	return len(p), nil
}
