package httpupgrade

import "net"

type connection struct {
	net.Conn
	remoteAddr net.Addr
}

func newConnection(conn net.Conn, remoteAddr net.Addr) *connection {
	return &connection{
		Conn:       conn,
		remoteAddr: remoteAddr,
	}
}

func (c *connection) RemoteAddr() net.Addr {
	return c.remoteAddr
}
