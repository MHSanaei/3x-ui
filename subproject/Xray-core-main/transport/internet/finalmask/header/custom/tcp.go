package custom

import (
	"bytes"
	"io"
	"net"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/common/errors"
)

type tcpCustomClient struct {
	clients []*TCPSequence
	servers []*TCPSequence
}

type tcpCustomClientConn struct {
	net.Conn
	header *tcpCustomClient

	auth bool
	wg   sync.WaitGroup
	once sync.Once
}

func NewConnClientTCP(c *TCPConfig, raw net.Conn) (net.Conn, error) {
	conn := &tcpCustomClientConn{
		Conn: raw,
		header: &tcpCustomClient{
			clients: c.Clients,
			servers: c.Servers,
		},
	}

	conn.wg.Add(1)

	return conn, nil
}

func (c *tcpCustomClientConn) TcpMaskConn() {}

func (c *tcpCustomClientConn) RawConn() net.Conn {
	// c.wg.Wait()

	return c.Conn
}

func (c *tcpCustomClientConn) Splice() bool {
	return true
}

func (c *tcpCustomClientConn) Read(p []byte) (n int, err error) {
	c.wg.Wait()

	if !c.auth {
		return 0, errors.New("header auth failed")
	}

	return c.Conn.Read(p)
}

func (c *tcpCustomClientConn) Write(p []byte) (n int, err error) {
	c.once.Do(func() {
		i := 0
		j := 0
		for i = range c.header.clients {
			if !writeSequence(c.Conn, c.header.clients[i]) {
				c.wg.Done()
				return
			}

			if j < len(c.header.servers) {
				if !readSequence(c.Conn, c.header.servers[j]) {
					c.wg.Done()
					return
				}
				j++
			}
		}

		for j < len(c.header.servers) {
			if !readSequence(c.Conn, c.header.servers[j]) {
				c.wg.Done()
				return
			}
			j++
		}

		c.auth = true
		c.wg.Done()
	})

	c.wg.Wait()

	if !c.auth {
		return 0, errors.New("header auth failed")
	}

	return c.Conn.Write(p)
}

type tcpCustomServer struct {
	clients []*TCPSequence
	servers []*TCPSequence
	errors  []*TCPSequence
}

type tcpCustomServerConn struct {
	net.Conn
	header *tcpCustomServer

	auth bool
	wg   sync.WaitGroup
	once sync.Once
}

func NewConnServerTCP(c *TCPConfig, raw net.Conn) (net.Conn, error) {
	conn := &tcpCustomServerConn{
		Conn: raw,
		header: &tcpCustomServer{
			clients: c.Clients,
			servers: c.Servers,
			errors:  c.Errors,
		},
	}

	conn.wg.Add(1)

	return conn, nil
}

func (c *tcpCustomServerConn) TcpMaskConn() {}

func (c *tcpCustomServerConn) RawConn() net.Conn {
	// c.wg.Wait()

	return c.Conn
}

func (c *tcpCustomServerConn) Splice() bool {
	return true
}

func (c *tcpCustomServerConn) Read(p []byte) (n int, err error) {
	c.once.Do(func() {
		i := 0
		j := 0
		for i = range c.header.clients {
			if !readSequence(c.Conn, c.header.clients[i]) {
				if i < len(c.header.errors) {
					writeSequence(c.Conn, c.header.errors[i])
				}
				c.wg.Done()
				return
			}

			if j < len(c.header.servers) {
				if !writeSequence(c.Conn, c.header.servers[j]) {
					c.wg.Done()
					return
				}
				j++
			}
		}

		for j < len(c.header.servers) {
			if !writeSequence(c.Conn, c.header.servers[j]) {
				c.wg.Done()
				return
			}
			j++
		}

		c.auth = true
		c.wg.Done()
	})

	c.wg.Wait()

	if !c.auth {
		return 0, errors.New("header auth failed")
	}

	return c.Conn.Read(p)
}

func (c *tcpCustomServerConn) Write(p []byte) (n int, err error) {
	c.wg.Wait()

	if !c.auth {
		return 0, errors.New("header auth failed")
	}

	return c.Conn.Write(p)
}

func readSequence(r io.Reader, sequence *TCPSequence) bool {
	for _, item := range sequence.Sequence {
		length := max(int(item.Rand), len(item.Packet))
		buf := make([]byte, length)
		n, err := io.ReadFull(r, buf)
		if err != nil {
			return false
		}
		if item.Rand > 0 && n != length {
			return false
		}
		if len(item.Packet) > 0 && !bytes.Equal(item.Packet, buf[:n]) {
			return false
		}
	}
	return true
}

func writeSequence(w io.Writer, sequence *TCPSequence) bool {
	var merged []byte
	for _, item := range sequence.Sequence {
		if item.DelayMax > 0 {
			if len(merged) > 0 {
				_, err := w.Write(merged)
				if err != nil {
					return false
				}
				merged = nil
			}
			time.Sleep(time.Duration(crypto.RandBetween(item.DelayMin, item.DelayMax)) * time.Millisecond)
		}
		if item.Rand > 0 {
			buf := make([]byte, item.Rand)
			crypto.RandBytesBetween(buf, byte(item.RandMin), byte(item.RandMax))
			merged = append(merged, buf...)
		} else {
			merged = append(merged, item.Packet...)
		}
	}
	if len(merged) > 0 {
		_, err := w.Write(merged)
		if err != nil {
			return false
		}
		merged = nil
	}
	return true
}
