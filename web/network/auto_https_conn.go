package network

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"sync"
)

type AutoHttpsConn struct {
	net.Conn

	firstBuf []byte
	bufStart int

	readRequestOnce sync.Once
}

func NewAutoHttpsConn(conn net.Conn) net.Conn {
	return &AutoHttpsConn{
		Conn: conn,
	}
}

func (c *AutoHttpsConn) readRequest() bool {
	c.firstBuf = make([]byte, 2048)
	n, err := c.Conn.Read(c.firstBuf)
	if err != nil {
		return false
	}
	c.firstBuf = c.firstBuf[:n]
	reader := bytes.NewReader(c.firstBuf)
	bufReader := bufio.NewReader(reader)
	request, err := http.ReadRequest(bufReader)
	if err != nil {
		return false
	}
	resp := &http.Response{
		StatusCode: http.StatusTemporaryRedirect,
		Header:     make(http.Header),
	}
	resp.Header.Set("Location", fmt.Sprintf("https://%v%v", request.Host, request.RequestURI))
	resp.Write(c.Conn)
	c.Close()
	return true
}

func (c *AutoHttpsConn) Read(buf []byte) (int, error) {
	var err error
	c.readRequestOnce.Do(func() {
		if !c.readRequest() {
			err = fmt.Errorf("failed to read HTTP request")
		}
	})
	if err != nil {
		return 0, err
	}

	if c.firstBuf != nil {
		n := copy(buf, c.firstBuf[c.bufStart:])
		c.bufStart += n
		if c.bufStart == len(c.firstBuf) {
			c.firstBuf = nil
		}
		return n, nil
	}

	return c.Conn.Read(buf)
}
