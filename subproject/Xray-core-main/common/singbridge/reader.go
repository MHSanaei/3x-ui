package singbridge

import (
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
)

var (
	_ buf.Reader        = (*Conn)(nil)
	_ buf.TimeoutReader = (*Conn)(nil)
	_ buf.Writer        = (*Conn)(nil)
)

type Conn struct {
	net.Conn
	writer N.VectorisedWriter
}

func NewConn(conn net.Conn) *Conn {
	writer, _ := bufio.CreateVectorisedWriter(conn)
	return &Conn{
		Conn:   conn,
		writer: writer,
	}
}

func (c *Conn) ReadMultiBuffer() (buf.MultiBuffer, error) {
	buffer, err := buf.ReadBuffer(c.Conn)
	if err != nil {
		return nil, err
	}
	return buf.MultiBuffer{buffer}, nil
}

func (c *Conn) ReadMultiBufferTimeout(duration time.Duration) (buf.MultiBuffer, error) {
	err := c.SetReadDeadline(time.Now().Add(duration))
	if err != nil {
		return nil, err
	}
	defer c.SetReadDeadline(time.Time{})
	return c.ReadMultiBuffer()
}

func (c *Conn) WriteMultiBuffer(bufferList buf.MultiBuffer) error {
	defer buf.ReleaseMulti(bufferList)
	if c.writer != nil {
		bytesList := make([][]byte, len(bufferList))
		for i, buffer := range bufferList {
			bytesList[i] = buffer.Bytes()
		}
		return common.Error(bufio.WriteVectorised(c.writer, bytesList))
	}
	// Since this conn is only used by tun, we don't force buffer writes to merge.
	for _, buffer := range bufferList {
		_, err := c.Conn.Write(buffer.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}
