package sudoku

import (
	"bufio"
	"io"
	"net"
	"sync"

	"github.com/xtls/xray-core/transport/internet/finalmask"
)

const ioBufferSize = 32 * 1024

var _ finalmask.TcpMaskConn = (*wrappedConn)(nil)

type streamDecoder interface {
	decodeChunk(in []byte, pending []byte) ([]byte, error)
	reset()
}

type streamReader struct {
	reader  *bufio.Reader
	rawBuf  []byte
	pending []byte
	decode  streamDecoder
	mu      sync.Mutex
}

func newStreamReader(raw net.Conn, decode streamDecoder) io.Reader {
	return &streamReader{
		reader:  bufio.NewReaderSize(raw, ioBufferSize),
		rawBuf:  make([]byte, ioBufferSize),
		pending: make([]byte, 0, 4096),
		decode:  decode,
	}
}

func (r *streamReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if n, ok := drainPending(p, &r.pending); ok {
		return n, nil
	}

	for len(r.pending) == 0 {
		nr, rErr := r.reader.Read(r.rawBuf)
		if nr > 0 {
			var dErr error
			r.pending, dErr = r.decode.decodeChunk(r.rawBuf[:nr], r.pending)
			if dErr != nil {
				return 0, dErr
			}
		}

		if rErr != nil {
			if rErr == io.EOF {
				r.decode.reset()
				if len(r.pending) > 0 {
					break
				}
			}
			return 0, rErr
		}
	}

	n, _ := drainPending(p, &r.pending)
	return n, nil
}

type streamWriter struct {
	conn   net.Conn
	encode func([]byte) ([]byte, error)
	mu     sync.Mutex
}

func newStreamWriter(raw net.Conn, encode func([]byte) ([]byte, error)) io.Writer {
	return &streamWriter{
		conn:   raw,
		encode: encode,
	}
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	encoded, err := w.encode(p)
	if err != nil {
		return 0, err
	}
	if err := writeAll(w.conn, encoded); err != nil {
		return 0, err
	}
	return len(p), nil
}

type wrappedConn struct {
	net.Conn
	reader io.Reader
	writer io.Writer
}

type closeWriteConn interface {
	CloseWrite() error
}

func newWrappedConn(raw net.Conn, reader io.Reader, writer io.Writer) net.Conn {
	return &wrappedConn{
		Conn:   raw,
		reader: reader,
		writer: writer,
	}
}

func (c *wrappedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *wrappedConn) Write(p []byte) (int, error) {
	return c.writer.Write(p)
}

func (c *wrappedConn) TcpMaskConn() {}

func (c *wrappedConn) RawConn() net.Conn {
	return c.Conn
}

func (c *wrappedConn) Splice() bool {
	// Sudoku transforms the entire stream; bypassing it would disable masking.
	return false
}

func (c *wrappedConn) CloseWrite() error {
	if raw, ok := c.Conn.(closeWriteConn); ok {
		return raw.CloseWrite()
	}
	return net.ErrClosed
}

func NewTCPConn(raw net.Conn, config *Config) (net.Conn, error) {
	reader, writer, err := newPureReaderWriter(raw, config)
	if err != nil {
		return nil, err
	}
	return newWrappedConn(raw, reader, writer), nil
}

func newPureReaderWriter(raw net.Conn, config *Config) (io.Reader, io.Writer, error) {
	tables, err := getTables(config)
	if err != nil {
		return nil, nil, err
	}

	pMin, pMax := normalizedPadding(config)
	c := newCodec(tables, pMin, pMax)
	return newStreamReader(raw, newHintStreamDecoder(tables)), newStreamWriter(raw, c.encode), nil
}

type hintStreamDecoder struct {
	tables     []*table
	tableIndex int
	hintBuf    []byte
}

func newHintStreamDecoder(tables []*table) *hintStreamDecoder {
	return &hintStreamDecoder{
		tables:  tables,
		hintBuf: make([]byte, 0, 4),
	}
}

func (d *hintStreamDecoder) decodeChunk(in []byte, pending []byte) ([]byte, error) {
	var err error
	d.hintBuf, pending, err = decodeBytes(d.tables, &d.tableIndex, in, d.hintBuf, pending)
	return pending, err
}

func (d *hintStreamDecoder) reset() {}

func drainPending(p []byte, pending *[]byte) (int, bool) {
	if len(*pending) == 0 {
		return 0, false
	}

	n := copy(p, *pending)
	if n >= len(*pending) {
		*pending = (*pending)[:0]
		return n, true
	}

	remaining := len(*pending) - n
	copy(*pending, (*pending)[n:])
	*pending = (*pending)[:remaining]
	return n, true
}

func writeAll(conn net.Conn, b []byte) error {
	for len(b) > 0 {
		n, err := conn.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}
