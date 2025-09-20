// Package network provides network utilities for the 3x-ui web panel,
// including automatic HTTP to HTTPS redirection functionality.
package network

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"sync"
)

// AutoHttpsConn wraps a net.Conn to provide automatic HTTP to HTTPS redirection.
// It intercepts the first read to detect HTTP requests and responds with a 307 redirect
// to the HTTPS equivalent URL. Subsequent reads work normally for HTTPS connections.
type AutoHttpsConn struct {
	net.Conn

	firstBuf []byte
	bufStart int

	readRequestOnce sync.Once
}

// NewAutoHttpsConn creates a new AutoHttpsConn that wraps the given connection.
// It enables automatic redirection of HTTP requests to HTTPS.
func NewAutoHttpsConn(conn net.Conn) net.Conn {
	return &AutoHttpsConn{
		Conn: conn,
	}
}

func (c *AutoHttpsConn) readRequest() bool {
	c.firstBuf = make([]byte, 2048)
	n, err := c.Conn.Read(c.firstBuf)
	c.firstBuf = c.firstBuf[:n]
	if err != nil {
		return false
	}
	reader := bytes.NewReader(c.firstBuf)
	bufReader := bufio.NewReader(reader)
	request, err := http.ReadRequest(bufReader)
	if err != nil {
		return false
	}
	resp := http.Response{
		Header: http.Header{},
	}
	resp.StatusCode = http.StatusTemporaryRedirect
	location := fmt.Sprintf("https://%v%v", request.Host, request.RequestURI)
	resp.Header.Set("Location", location)
	resp.Write(c.Conn)
	c.Close()
	c.firstBuf = nil
	return true
}

// Read implements the net.Conn Read method with automatic HTTPS redirection.
// On the first read, it checks if the request is HTTP and redirects to HTTPS if so.
// Subsequent reads work normally.
func (c *AutoHttpsConn) Read(buf []byte) (int, error) {
	c.readRequestOnce.Do(func() {
		c.readRequest()
	})

	if c.firstBuf != nil {
		n := copy(buf, c.firstBuf[c.bufStart:])
		c.bufStart += n
		if c.bufStart >= len(c.firstBuf) {
			c.firstBuf = nil
		}
		return n, nil
	}

	return c.Conn.Read(buf)
}
