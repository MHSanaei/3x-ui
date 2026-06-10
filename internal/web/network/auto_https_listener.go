package network

import "net"

// AutoHttpsListener wraps a net.Listener to provide automatic HTTPS redirection.
// It returns AutoHttpsConn connections that handle HTTP to HTTPS redirection.
type AutoHttpsListener struct {
	net.Listener
}

// NewAutoHttpsListener creates a new AutoHttpsListener that wraps the given listener.
// It enables automatic redirection of HTTP requests to HTTPS for all accepted connections.
func NewAutoHttpsListener(listener net.Listener) net.Listener {
	return &AutoHttpsListener{
		Listener: listener,
	}
}

// Accept implements the net.Listener Accept method.
// It accepts connections and wraps them with AutoHttpsConn for HTTPS redirection.
func (l *AutoHttpsListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewAutoHttpsConn(conn), nil
}
