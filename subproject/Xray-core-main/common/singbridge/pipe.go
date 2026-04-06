package singbridge

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing/common/bufio"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/transport"
)

func CopyConn(ctx context.Context, inboundConn net.Conn, link *transport.Link, serverConn net.Conn) error {
	conn := &PipeConnWrapper{
		W:    link.Writer,
		Conn: inboundConn,
	}
	if ir, ok := link.Reader.(io.Reader); ok {
		conn.R = ir
	} else {
		conn.R = &buf.BufferedReader{Reader: link.Reader}
	}
	return ReturnError(bufio.CopyConn(ctx, conn, serverConn))
}

type PipeConnWrapper struct {
	R io.Reader
	W buf.Writer
	net.Conn
}

func (w *PipeConnWrapper) Close() error {
	return nil
}

// This Read implemented a timeout to avoid goroutine leak.
// as a temporarily solution
func (w *PipeConnWrapper) Read(b []byte) (n int, err error) {
	type readResult struct {
		n   int
		err error
	}
	c := make(chan readResult, 1)
	go func() {
		n, err := w.R.Read(b)
		c <- readResult{n: n, err: err}
	}()
	select {
	case result := <-c:
		return result.n, result.err
	case <-time.After(300 * time.Second):
		common.Close(w.R)
		common.Interrupt(w.R)
		return 0, buf.ErrReadTimeout
	}
}

func (w *PipeConnWrapper) Write(p []byte) (n int, err error) {
	n = len(p)
	var mb buf.MultiBuffer
	pLen := len(p)
	for pLen > 0 {
		buffer := buf.New()
		if pLen > buf.Size {
			_, err = buffer.Write(p[:buf.Size])
			p = p[buf.Size:]
		} else {
			buffer.Write(p)
		}
		pLen -= int(buffer.Len())
		mb = append(mb, buffer)
	}
	err = w.W.WriteMultiBuffer(mb)
	if err != nil {
		n = 0
		buf.ReleaseMulti(mb)
	}
	return
}
