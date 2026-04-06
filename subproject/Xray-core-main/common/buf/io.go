package buf

import (
	"context"
	"io"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// Reader extends io.Reader with MultiBuffer.
type Reader interface {
	// ReadMultiBuffer reads content from underlying reader, and put it into a MultiBuffer.
	ReadMultiBuffer() (MultiBuffer, error)
}

// ErrReadTimeout is an error that happens with IO timeout.
var ErrReadTimeout = errors.New("IO timeout")

// TimeoutReader is a reader that returns error if Read() operation takes longer than the given timeout.
type TimeoutReader interface {
	Reader
	ReadMultiBufferTimeout(time.Duration) (MultiBuffer, error)
}

type TimeoutWrapperReader struct {
	Reader
	stats.Counter
	mb   MultiBuffer
	err  error
	done chan struct{}
}

func (r *TimeoutWrapperReader) ReadMultiBuffer() (MultiBuffer, error) {
	if r.done != nil {
		<-r.done
		r.done = nil
		if r.Counter != nil {
			r.Counter.Add(int64(r.mb.Len()))
		}
		return r.mb, r.err
	}
	r.mb, r.err = r.Reader.ReadMultiBuffer()
	if r.Counter != nil {
		r.Counter.Add(int64(r.mb.Len()))
	}
	return r.mb, r.err
}

func (r *TimeoutWrapperReader) ReadMultiBufferTimeout(duration time.Duration) (MultiBuffer, error) {
	if r.done == nil {
		r.done = make(chan struct{})
		go func() {
			r.mb, r.err = r.Reader.ReadMultiBuffer()
			close(r.done)
		}()
	}
	timeout := make(chan struct{})
	go func() {
		time.Sleep(duration)
		close(timeout)
	}()
	select {
	case <-r.done:
		r.done = nil
		if r.Counter != nil {
			r.Counter.Add(int64(r.mb.Len()))
		}
		return r.mb, r.err
	case <-timeout:
		return nil, nil
	}
}

// Writer extends io.Writer with MultiBuffer.
type Writer interface {
	// WriteMultiBuffer writes a MultiBuffer into underlying writer.
	WriteMultiBuffer(MultiBuffer) error
}

// WriteAllBytes ensures all bytes are written into the given writer.
func WriteAllBytes(writer io.Writer, payload []byte, c stats.Counter) error {
	wc := 0
	defer func() {
		if c != nil {
			c.Add(int64(wc))
		}
	}()

	for len(payload) > 0 {
		n, err := writer.Write(payload)
		wc += n
		if err != nil {
			return err
		}
		payload = payload[n:]
	}
	return nil
}

func isPacketReader(reader io.Reader) bool {
	_, ok := reader.(net.PacketConn)
	return ok
}

// NewReader creates a new Reader.
// The Reader instance doesn't take the ownership of reader.
func NewReader(reader io.Reader) Reader {
	if mr, ok := reader.(Reader); ok {
		return mr
	}

	if isPacketReader(reader) {
		return &PacketReader{
			Reader: reader,
		}
	}

	_, isFile := reader.(*os.File)
	if !isFile && useReadv {
		if sc, ok := reader.(syscall.Conn); ok {
			rawConn, err := sc.SyscallConn()
			if err != nil {
				errors.LogInfoInner(context.Background(), err, "failed to get sysconn")
			} else {
				var counter stats.Counter

				if statConn, ok := reader.(*stat.CounterConnection); ok {
					reader = statConn.Connection
					counter = statConn.ReadCounter
				}
				return NewReadVReader(reader, rawConn, counter)
			}
		}
	}

	return &SingleReader{
		Reader: reader,
	}
}

// NewPacketReader creates a new PacketReader based on the given reader.
func NewPacketReader(reader io.Reader) Reader {
	if mr, ok := reader.(Reader); ok {
		return mr
	}

	return &PacketReader{
		Reader: reader,
	}
}

func isPacketWriter(writer io.Writer) bool {
	if _, ok := writer.(net.PacketConn); ok {
		return true
	}

	// If the writer doesn't implement syscall.Conn, it is probably not a TCP connection.
	if _, ok := writer.(syscall.Conn); !ok {
		return true
	}
	return false
}

// NewWriter creates a new Writer.
func NewWriter(writer io.Writer) Writer {
	if mw, ok := writer.(Writer); ok {
		return mw
	}

	iConn := writer
	if statConn, ok := writer.(*stat.CounterConnection); ok {
		iConn = statConn.Connection
	}

	if isPacketWriter(iConn) {
		return &SequentialWriter{
			Writer: writer,
		}
	}

	var counter stats.Counter

	if statConn, ok := writer.(*stat.CounterConnection); ok {
		counter = statConn.WriteCounter
	}
	return &BufferToBytesWriter{
		Writer:  iConn,
		counter: counter,
	}
}
