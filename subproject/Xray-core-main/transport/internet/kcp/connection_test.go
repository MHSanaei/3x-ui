package kcp_test

import (
	"io"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	. "github.com/xtls/xray-core/transport/internet/kcp"
)

type NoOpCloser int

func (NoOpCloser) Close() error {
	return nil
}

func TestConnectionReadTimeout(t *testing.T) {
	conn := NewConnection(ConnMetadata{Conversation: 1}, buf.DiscardBytes, NoOpCloser(0), &Config{})
	conn.SetReadDeadline(time.Now().Add(time.Second))

	b := make([]byte, 1024)
	nBytes, err := conn.Read(b)
	if nBytes != 0 || err == nil {
		t.Error("unexpected read: ", nBytes, err)
	}

	conn.Terminate()
}

func TestConnectionInterface(t *testing.T) {
	_ = (io.Writer)(new(Connection))
	_ = (io.Reader)(new(Connection))
	_ = (buf.Reader)(new(Connection))
	_ = (buf.Writer)(new(Connection))
}
