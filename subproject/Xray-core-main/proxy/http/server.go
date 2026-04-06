package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	http_proto "github.com/xtls/xray-core/common/protocol/http"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/proxy"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// Server is an HTTP proxy server.
type Server struct {
	config        *ServerConfig
	policyManager policy.Manager
}

// NewServer creates a new HTTP inbound handler.
func NewServer(ctx context.Context, config *ServerConfig) (*Server, error) {
	v := core.MustFromContext(ctx)
	s := &Server{
		config:        config,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	return s, nil
}

func (s *Server) policy() policy.Session {
	config := s.config
	p := s.policyManager.ForLevel(config.UserLevel)
	return p
}

// Network implements proxy.Inbound.
func (*Server) Network() []net.Network {
	return []net.Network{net.Network_TCP, net.Network_UNIX}
}

func isTimeout(err error) bool {
	nerr, ok := errors.Cause(err).(net.Error)
	return ok && nerr.Timeout()
}

func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

type readerOnly struct {
	io.Reader
}

func (s *Server) Process(ctx context.Context, network net.Network, conn stat.Connection, dispatcher routing.Dispatcher) error {
	return s.ProcessWithFirstbyte(ctx, network, conn, dispatcher)
}

// Firstbyte is for forwarded conn from SOCKS inbound
// Because it needs first byte to choose protocol
// We need to add it back
// Other parts are the same as the process function
func (s *Server) ProcessWithFirstbyte(ctx context.Context, network net.Network, conn stat.Connection, dispatcher routing.Dispatcher, firstbyte ...byte) error {
	inbound := session.InboundFromContext(ctx)
	inbound.Name = "http"
	inbound.CanSpliceCopy = 2
	inbound.User = &protocol.MemoryUser{
		Level: s.config.UserLevel,
	}
	if !proxy.IsRAWTransportWithoutSecurity(conn) {
		inbound.CanSpliceCopy = 3
	}
	var reader *bufio.Reader
	if len(firstbyte) > 0 {
		readerWithoutFirstbyte := bufio.NewReaderSize(readerOnly{conn}, buf.Size)
		multiReader := io.MultiReader(bytes.NewReader(firstbyte), readerWithoutFirstbyte)
		reader = bufio.NewReaderSize(multiReader, buf.Size)
	} else {
		reader = bufio.NewReaderSize(readerOnly{conn}, buf.Size)
	}

Start:
	if err := conn.SetReadDeadline(time.Now().Add(s.policy().Timeouts.Handshake)); err != nil {
		errors.LogInfoInner(ctx, err, "failed to set read deadline")
	}

	request, err := http.ReadRequest(reader)
	if err != nil {
		trace := errors.New("failed to read http request").Base(err)
		if errors.Cause(err) != io.EOF && !isTimeout(errors.Cause(err)) {
			trace.AtWarning()
		}
		return trace
	}

	if len(s.config.Accounts) > 0 {
		user, pass, ok := parseBasicAuth(request.Header.Get("Proxy-Authorization"))
		if !ok || !s.config.HasAccount(user, pass) {
			return common.Error2(conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"proxy\"\r\n\r\n")))
		}
		if inbound != nil {
			inbound.User.Email = user
		}
	}

	errors.LogInfo(ctx, "request to Method [", request.Method, "] Host [", request.Host, "] with URL [", request.URL, "]")
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		errors.LogDebugInner(ctx, err, "failed to clear read deadline")
	}

	defaultPort := net.Port(80)
	if strings.EqualFold(request.URL.Scheme, "https") {
		defaultPort = net.Port(443)
	}
	host := request.Host
	if host == "" {
		host = request.URL.Host
	}
	dest, err := http_proto.ParseHost(host, defaultPort)
	if err != nil {
		return errors.New("malformed proxy host: ", host).AtWarning().Base(err)
	}
	ctx = log.ContextWithAccessMessage(ctx, &log.AccessMessage{
		From:   conn.RemoteAddr(),
		To:     request.URL,
		Status: log.AccessAccepted,
		Reason: "",
	})

	if strings.EqualFold(request.Method, "CONNECT") {
		return s.handleConnect(ctx, request, reader, conn, dest, dispatcher, inbound)
	}

	keepAlive := (strings.TrimSpace(strings.ToLower(request.Header.Get("Proxy-Connection"))) == "keep-alive")

	err = s.handlePlainHTTP(ctx, request, conn, dest, dispatcher)
	if err == errWaitAnother {
		if keepAlive {
			goto Start
		}
		err = nil
	}

	return err
}

func (s *Server) handleConnect(ctx context.Context, _ *http.Request, buffer *bufio.Reader, conn stat.Connection, dest net.Destination, dispatcher routing.Dispatcher, inbound *session.Inbound) error {
	_, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		return errors.New("failed to write back OK response").Base(err)
	}

	reader := buf.NewReader(conn)
	if buffer.Buffered() > 0 {
		payload, err := buf.ReadFrom(io.LimitReader(buffer, int64(buffer.Buffered())))
		if err != nil {
			return err
		}
		reader = &buf.BufferedReader{Reader: reader, Buffer: payload}
		buffer = nil
	}

	if inbound.CanSpliceCopy == 2 {
		inbound.CanSpliceCopy = 1
	}
	if err := dispatcher.DispatchLink(ctx, dest, &transport.Link{
		Reader: reader,
		Writer: buf.NewWriter(conn)},
	); err != nil {
		return errors.New("failed to dispatch request").Base(err)
	}
	return nil
}

var errWaitAnother = errors.New("keep alive")

func (s *Server) handlePlainHTTP(ctx context.Context, request *http.Request, writer io.Writer, dest net.Destination, dispatcher routing.Dispatcher) error {
	if !s.config.AllowTransparent && request.URL.Host == "" {
		// RFC 2068 (HTTP/1.1) requires URL to be absolute URL in HTTP proxy.
		response := &http.Response{
			Status:        "Bad Request",
			StatusCode:    400,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        http.Header(make(map[string][]string)),
			Body:          nil,
			ContentLength: 0,
			Close:         true,
		}
		response.Header.Set("Proxy-Connection", "close")
		response.Header.Set("Connection", "close")
		return response.Write(writer)
	}

	if len(request.URL.Host) > 0 {
		request.Host = request.URL.Host
	}
	http_proto.RemoveHopByHopHeaders(request.Header)

	// Prevent UA from being set to golang's default ones
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", "")
	}

	content := &session.Content{
		Protocol: "http/1.1",
	}

	content.SetAttribute(":method", strings.ToUpper(request.Method))
	content.SetAttribute(":path", request.URL.Path)
	for key := range request.Header {
		value := request.Header.Get(key)
		content.SetAttribute(strings.ToLower(key), value)
	}

	ctx = session.ContextWithContent(ctx, content)

	link, err := dispatcher.Dispatch(ctx, dest)
	if err != nil {
		return err
	}

	// Plain HTTP request is not a stream. The request always finishes before response. Hense request has to be closed later.
	defer common.Close(link.Writer)
	var result error = errWaitAnother

	requestDone := func() error {
		request.Header.Set("Connection", "close")

		requestWriter := buf.NewBufferedWriter(link.Writer)
		common.Must(requestWriter.SetBuffered(false))
		if err := request.Write(requestWriter); err != nil {
			return errors.New("failed to write whole request").Base(err).AtWarning()
		}
		return nil
	}

	responseDone := func() error {
		responseReader := bufio.NewReaderSize(&buf.BufferedReader{Reader: link.Reader}, buf.Size)
		response, err := readResponseAndHandle100Continue(responseReader, request, writer)
		if err == nil {
			http_proto.RemoveHopByHopHeaders(response.Header)
			if response.ContentLength >= 0 {
				response.Header.Set("Proxy-Connection", "keep-alive")
				response.Header.Set("Connection", "keep-alive")
				response.Header.Set("Keep-Alive", "timeout=60")
				response.Close = false
			} else {
				response.Close = true
				result = nil
			}
			defer response.Body.Close()
		} else {
			errors.LogWarningInner(ctx, err, "failed to read response from ", request.Host)
			response = &http.Response{
				Status:        "Service Unavailable",
				StatusCode:    503,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header(make(map[string][]string)),
				Body:          nil,
				ContentLength: 0,
				Close:         true,
			}
			response.Header.Set("Connection", "close")
			response.Header.Set("Proxy-Connection", "close")
		}
		if err := response.Write(writer); err != nil {
			return errors.New("failed to write response").Base(err).AtWarning()
		}
		return nil
	}

	if err := task.Run(ctx, requestDone, responseDone); err != nil {
		common.Interrupt(link.Reader)
		common.Interrupt(link.Writer)
		return errors.New("connection ends").Base(err)
	}

	return result
}

// Sometimes, server might send 1xx response to client
// it should not be processed by http proxy handler, just forward it to client
func readResponseAndHandle100Continue(r *bufio.Reader, req *http.Request, writer io.Writer) (*http.Response, error) {
	// have a little look of response
	peekBytes, err := r.Peek(56)
	if err == nil || err == bufio.ErrBufferFull {
		str := string(peekBytes)
		ResponseLine := strings.Split(str, "\r\n")[0]
		_, status, _ := strings.Cut(ResponseLine, " ")
		// only handle 1xx response
		if strings.HasPrefix(status, "1") {
			ResponseHeader1xx := []byte{}
			// read until \r\n\r\n (end of http response header)
			for {
				data, err := r.ReadSlice('\n')
				if err != nil {
					return nil, errors.New("failed to read http 1xx response").Base(err)
				}
				ResponseHeader1xx = append(ResponseHeader1xx, data...)
				if bytes.Equal(ResponseHeader1xx[len(ResponseHeader1xx)-4:], []byte{'\r', '\n', '\r', '\n'}) {
					break
				}
				if len(ResponseHeader1xx) > 1024 {
					return nil, errors.New("too big http 1xx response")
				}
			}
			writer.Write(ResponseHeader1xx)
		}
	}
	return http.ReadResponse(r, req)
}

func init() {
	common.Must(common.RegisterConfig((*ServerConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewServer(ctx, config.(*ServerConfig))
	}))
}
