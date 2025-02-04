package network

import "net"

type AutoHttpsListener struct {
	net.Listener
}

func NewAutoHttpsListener(listener net.Listener) net.Listener {
	return &AutoHttpsListener{
		Listener: listener,
	}
}

func (l *AutoHttpsListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewAutoHttpsConn(conn), nil
}
