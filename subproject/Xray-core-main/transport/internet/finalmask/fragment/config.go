package fragment

import "net"

func (c *Config) TCP() {
}

func (c *Config) WrapConnClient(raw net.Conn) (net.Conn, error) {
	return NewConnClient(c, raw, false)
}

func (c *Config) WrapConnServer(raw net.Conn) (net.Conn, error) {
	return NewConnServer(c, raw, true)
}
