package sudoku

import (
	"io"
	"net"
	"sync"
	"time"
)

type udpConn struct {
	conn   net.PacketConn
	tables []*table
	pMin   int
	pMax   int

	readBuf []byte

	readMu  sync.Mutex
	writeMu sync.Mutex
}

func NewUDPConn(raw net.PacketConn, config *Config) (net.PacketConn, error) {
	tables, err := getTables(config)
	if err != nil {
		return nil, err
	}

	pMin, pMax := normalizedPadding(config)
	return &udpConn{
		conn:    raw,
		tables:  tables,
		pMin:    pMin,
		pMax:    pMax,
		readBuf: make([]byte, 65535),
	}, nil
}

func (c *udpConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	n, addr, err = c.conn.ReadFrom(c.readBuf)
	if err != nil {
		return n, addr, err
	}

	decoded := make([]byte, 0, n/4+1)
	hints := make([]byte, 0, 4)
	tableIndex := 0
	hints, decoded, err = decodeBytes(c.tables, &tableIndex, c.readBuf[:n], hints, decoded)
	if err != nil {
		return 0, addr, err
	}
	if len(hints) != 0 {
		return 0, addr, io.ErrUnexpectedEOF
	}
	if len(p) < len(decoded) {
		return 0, addr, io.ErrShortBuffer
	}
	copy(p, decoded)
	return len(decoded), addr, nil
}

func (c *udpConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// UDP decoding restarts at table 0 for every datagram, so encoding must do the same.
	encoded, err := newCodec(c.tables, c.pMin, c.pMax).encode(p)
	if err != nil {
		return 0, err
	}

	nn, err := c.conn.WriteTo(encoded, addr)
	if err != nil {
		return 0, err
	}
	if nn != len(encoded) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}

func (c *udpConn) Close() error {
	return c.conn.Close()
}

func (c *udpConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *udpConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *udpConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *udpConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
