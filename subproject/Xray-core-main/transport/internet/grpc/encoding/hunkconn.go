package encoding

import (
	"context"
	"io"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/common/signal/done"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type HunkConn interface {
	Context() context.Context
	Send(*Hunk) error
	Recv() (*Hunk, error)
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

type StreamCloser interface {
	CloseSend() error
}

type HunkReaderWriter struct {
	hc     HunkConn
	cancel context.CancelFunc
	done   *done.Instance

	buf   []byte
	index int
}

func NewHunkReadWriter(hc HunkConn, cancel context.CancelFunc) *HunkReaderWriter {
	return &HunkReaderWriter{hc, cancel, done.New(), nil, 0}
}

func NewHunkConn(hc HunkConn, cancel context.CancelFunc) net.Conn {
	var rAddr net.Addr
	pr, ok := peer.FromContext(hc.Context())
	if ok {
		rAddr = pr.Addr
	} else {
		rAddr = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}

	md, ok := metadata.FromIncomingContext(hc.Context())
	if ok {
		header := md.Get("x-real-ip")
		if len(header) > 0 {
			realip := net.ParseAddress(header[0])
			if realip.Family().IsIP() {
				rAddr = &net.TCPAddr{
					IP:   realip.IP(),
					Port: 0,
				}
			}
		}
	}
	wrc := NewHunkReadWriter(hc, cancel)
	return cnc.NewConnection(
		cnc.ConnectionInput(wrc),
		cnc.ConnectionOutput(wrc),
		cnc.ConnectionOnClose(wrc),
		cnc.ConnectionRemoteAddr(rAddr),
	)
}

func (h *HunkReaderWriter) forceFetch() error {
	hunk, err := h.hc.Recv()
	if err != nil {
		if err == io.EOF {
			return err
		}

		return errors.New("failed to fetch hunk from gRPC tunnel").Base(err)
	}

	h.buf = hunk.Data
	h.index = 0

	return nil
}

func (h *HunkReaderWriter) Read(buf []byte) (int, error) {
	if h.done.Done() {
		return 0, io.EOF
	}

	if h.index >= len(h.buf) {
		if err := h.forceFetch(); err != nil {
			return 0, err
		}
	}
	n := copy(buf, h.buf[h.index:])
	h.index += n

	return n, nil
}

func (h *HunkReaderWriter) ReadMultiBuffer() (buf.MultiBuffer, error) {
	if h.done.Done() {
		return nil, io.EOF
	}
	if h.index >= len(h.buf) {
		if err := h.forceFetch(); err != nil {
			return nil, err
		}
	}

	if cap(h.buf) >= buf.Size {
		b := h.buf
		h.index = len(h.buf)
		return buf.MultiBuffer{buf.NewExisted(b)}, nil
	}

	b := buf.New()
	_, err := b.ReadFrom(h)
	if err != nil {
		return nil, err
	}
	return buf.MultiBuffer{b}, nil
}

func (h *HunkReaderWriter) Write(buf []byte) (int, error) {
	if h.done.Done() {
		return 0, io.ErrClosedPipe
	}

	err := h.hc.Send(&Hunk{Data: buf[:]})
	if err != nil {
		return 0, errors.New("failed to send data over gRPC tunnel").Base(err)
	}
	return len(buf), nil
}

func (h *HunkReaderWriter) Close() error {
	if h.cancel != nil {
		h.cancel()
	}
	if sc, match := h.hc.(StreamCloser); match {
		return sc.CloseSend()
	}

	return h.done.Close()
}
