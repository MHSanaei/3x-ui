package http

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
)

type version byte

const (
	HTTP1 version = iota
	HTTP2
)

type SniffHeader struct {
	version version
	host    string
}

func (h *SniffHeader) Protocol() string {
	switch h.version {
	case HTTP1:
		return "http1"
	case HTTP2:
		return "http2"
	default:
		return "unknown"
	}
}

func (h *SniffHeader) Domain() string {
	return h.host
}

var (
	methods = [...]string{"get", "post", "head", "put", "delete", "options", "connect"}

	errNotHTTPMethod = errors.New("not an HTTP method")
)

func beginWithHTTPMethod(b []byte) error {
	for _, m := range &methods {
		if len(b) >= len(m) && strings.EqualFold(string(b[:len(m)]), m) {
			return nil
		}

		if len(b) < len(m) {
			return common.ErrNoClue
		}
	}

	return errNotHTTPMethod
}

func SniffHTTP(b []byte, c context.Context) (*SniffHeader, error) {
	content := session.ContentFromContext(c)
	ShouldSniffAttr := true
	// If content.Attributes have information, that means it comes from HTTP inbound PlainHTTP mode.
	// It will set attributes, so skip it.
	if content == nil || len(content.Attributes) != 0 {
		ShouldSniffAttr = false
	}
	if err := beginWithHTTPMethod(b); err != nil {
		return nil, err
	}

	sh := &SniffHeader{
		version: HTTP1,
	}

	headers := bytes.Split(b, []byte{'\n'})
	for i := 1; i < len(headers); i++ {
		header := headers[i]
		if len(header) == 0 {
			break
		}
		parts := bytes.SplitN(header, []byte{':'}, 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(string(parts[0]))
		value := string(bytes.TrimSpace(parts[1]))
		if ShouldSniffAttr {
			content.SetAttribute(key, value) // Put header in attribute
		}
		if key == "host" {
			rawHost := strings.ToLower(value)
			dest, err := ParseHost(rawHost, net.Port(80))
			if err != nil {
				return nil, err
			}
			sh.host = dest.Address.String()
		}
	}
	// Parse request line
	// Request line is like this
	// "GET /homo/114514 HTTP/1.1"
	if len(headers) > 0 && ShouldSniffAttr {
		RequestLineParts := bytes.Split(headers[0], []byte{' '})
		if len(RequestLineParts) == 3 {
			content.SetAttribute(":method", string(RequestLineParts[0]))
			content.SetAttribute(":path", string(RequestLineParts[1]))
		}
	}

	if len(sh.host) > 0 {
		return sh, nil
	}

	return nil, common.ErrNoClue
}
