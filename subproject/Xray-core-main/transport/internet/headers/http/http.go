package http

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
)

const (
	// CRLF is the line ending in HTTP header
	CRLF = "\r\n"

	// ENDING is the double line ending between HTTP header and body.
	ENDING = CRLF + CRLF

	// max length of HTTP header. Safety precaution for DDoS attack.
	maxHeaderLength = 8192
)

var (
	ErrHeaderToLong = errors.New("Header too long.")

	ErrHeaderMisMatch = errors.New("Header Mismatch.")
)

type Reader interface {
	Read(io.Reader) (*buf.Buffer, error)
}

type Writer interface {
	Write(io.Writer) error
}

type NoOpReader struct{}

func (NoOpReader) Read(io.Reader) (*buf.Buffer, error) {
	return nil, nil
}

type NoOpWriter struct{}

func (NoOpWriter) Write(io.Writer) error {
	return nil
}

type HeaderReader struct {
	req            *http.Request
	expectedHeader *RequestConfig
}

func (h *HeaderReader) ExpectThisRequest(expectedHeader *RequestConfig) *HeaderReader {
	h.expectedHeader = expectedHeader
	return h
}

func (h *HeaderReader) Read(reader io.Reader) (*buf.Buffer, error) {
	buffer := buf.New()
	totalBytes := int32(0)
	endingDetected := false

	var headerBuf bytes.Buffer

	for totalBytes < maxHeaderLength {
		_, err := buffer.ReadFrom(reader)
		if err != nil {
			buffer.Release()
			return nil, err
		}
		if n := bytes.Index(buffer.Bytes(), []byte(ENDING)); n != -1 {
			headerBuf.Write(buffer.BytesRange(0, int32(n+len(ENDING))))
			buffer.Advance(int32(n + len(ENDING)))
			endingDetected = true
			break
		}
		lenEnding := int32(len(ENDING))
		if buffer.Len() >= lenEnding {
			totalBytes += buffer.Len() - lenEnding
			headerBuf.Write(buffer.BytesRange(0, buffer.Len()-lenEnding))
			leftover := buffer.BytesFrom(-lenEnding)
			buffer.Clear()
			copy(buffer.Extend(lenEnding), leftover)

			if _, err := readRequest(bufio.NewReader(bytes.NewReader(headerBuf.Bytes()))); err != io.ErrUnexpectedEOF {
				return nil, err
			}
		}
	}

	if !endingDetected {
		buffer.Release()
		return nil, ErrHeaderToLong
	}

	if h.expectedHeader == nil {
		if buffer.IsEmpty() {
			buffer.Release()
			return nil, nil
		}
		return buffer, nil
	}

	// Parse the request
	if req, err := readRequest(bufio.NewReader(bytes.NewReader(headerBuf.Bytes()))); err != nil {
		return nil, err
	} else {
		h.req = req
	}

	// Check req
	path := h.req.URL.Path
	hasThisURI := false
	for _, u := range h.expectedHeader.Uri {
		if u == path {
			hasThisURI = true
		}
	}

	if !hasThisURI {
		return nil, ErrHeaderMisMatch
	}

	if buffer.IsEmpty() {
		buffer.Release()
		return nil, nil
	}

	return buffer, nil
}

type HeaderWriter struct {
	header *buf.Buffer
}

func NewHeaderWriter(header *buf.Buffer) *HeaderWriter {
	return &HeaderWriter{
		header: header,
	}
}

func (w *HeaderWriter) Write(writer io.Writer) error {
	if w.header == nil {
		return nil
	}
	err := buf.WriteAllBytes(writer, w.header.Bytes(), nil)
	w.header.Release()
	w.header = nil
	return err
}

type Conn struct {
	net.Conn

	readBuffer          *buf.Buffer
	oneTimeReader       Reader
	oneTimeWriter       Writer
	errorWriter         Writer
	errorMismatchWriter Writer
	errorTooLongWriter  Writer
	errReason           error
}

func NewConn(conn net.Conn, reader Reader, writer Writer, errorWriter Writer, errorMismatchWriter Writer, errorTooLongWriter Writer) *Conn {
	return &Conn{
		Conn:                conn,
		oneTimeReader:       reader,
		oneTimeWriter:       writer,
		errorWriter:         errorWriter,
		errorMismatchWriter: errorMismatchWriter,
		errorTooLongWriter:  errorTooLongWriter,
	}
}

func (c *Conn) Read(b []byte) (int, error) {
	if c.oneTimeReader != nil {
		buffer, err := c.oneTimeReader.Read(c.Conn)
		if err != nil {
			c.errReason = err
			return 0, err
		}
		c.readBuffer = buffer
		c.oneTimeReader = nil
	}

	if !c.readBuffer.IsEmpty() {
		nBytes, _ := c.readBuffer.Read(b)
		if c.readBuffer.IsEmpty() {
			c.readBuffer.Release()
			c.readBuffer = nil
		}
		return nBytes, nil
	}

	return c.Conn.Read(b)
}

// Write implements io.Writer.
func (c *Conn) Write(b []byte) (int, error) {
	if c.oneTimeWriter != nil {
		err := c.oneTimeWriter.Write(c.Conn)
		c.oneTimeWriter = nil
		if err != nil {
			return 0, err
		}
	}

	return c.Conn.Write(b)
}

// Close implements net.Conn.Close().
func (c *Conn) Close() error {
	if c.oneTimeWriter != nil && c.errorWriter != nil {
		// Connection is being closed but header wasn't sent. This means the client request
		// is probably not valid. Sending back a server error header in this case.

		// Write response based on error reason
		switch c.errReason {
		case ErrHeaderMisMatch:
			c.errorMismatchWriter.Write(c.Conn)
		case ErrHeaderToLong:
			c.errorTooLongWriter.Write(c.Conn)
		default:
			c.errorWriter.Write(c.Conn)
		}
	}

	return c.Conn.Close()
}

func formResponseHeader(config *ResponseConfig) *HeaderWriter {
	header := buf.New()
	common.Must2(header.WriteString(strings.Join([]string{config.GetFullVersion(), config.GetStatusValue().Code, config.GetStatusValue().Reason}, " ")))
	common.Must2(header.WriteString(CRLF))

	headers := config.PickHeaders()
	for _, h := range headers {
		common.Must2(header.WriteString(h))
		common.Must2(header.WriteString(CRLF))
	}
	if !config.HasHeader("Date") {
		common.Must2(header.WriteString("Date: "))
		common.Must2(header.WriteString(time.Now().Format(http.TimeFormat)))
		common.Must2(header.WriteString(CRLF))
	}
	common.Must2(header.WriteString(CRLF))
	return &HeaderWriter{
		header: header,
	}
}

type Authenticator struct {
	config *Config
}

func (a Authenticator) GetClientWriter() *HeaderWriter {
	header := buf.New()
	config := a.config.Request
	common.Must2(header.WriteString(strings.Join([]string{config.GetMethodValue(), config.PickURI(), config.GetFullVersion()}, " ")))
	common.Must2(header.WriteString(CRLF))

	headers := config.PickHeaders()
	for _, h := range headers {
		common.Must2(header.WriteString(h))
		common.Must2(header.WriteString(CRLF))
	}
	common.Must2(header.WriteString(CRLF))
	return &HeaderWriter{
		header: header,
	}
}

func (a Authenticator) GetServerWriter() *HeaderWriter {
	return formResponseHeader(a.config.Response)
}

func (a Authenticator) Client(conn net.Conn) net.Conn {
	if a.config.Request == nil && a.config.Response == nil {
		return conn
	}
	var reader Reader = NoOpReader{}
	if a.config.Request != nil {
		reader = new(HeaderReader)
	}

	var writer Writer = NoOpWriter{}
	if a.config.Response != nil {
		writer = a.GetClientWriter()
	}
	return NewConn(conn, reader, writer, NoOpWriter{}, NoOpWriter{}, NoOpWriter{})
}

func (a Authenticator) Server(conn net.Conn) net.Conn {
	if a.config.Request == nil && a.config.Response == nil {
		return conn
	}
	return NewConn(conn, new(HeaderReader).ExpectThisRequest(a.config.Request), a.GetServerWriter(),
		formResponseHeader(resp400),
		formResponseHeader(resp404),
		formResponseHeader(resp400))
}

func NewAuthenticator(ctx context.Context, config *Config) (Authenticator, error) {
	return Authenticator{
		config: config,
	}, nil
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewAuthenticator(ctx, config.(*Config))
	}))
}
