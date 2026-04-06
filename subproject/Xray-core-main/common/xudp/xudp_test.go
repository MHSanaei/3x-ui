package xudp

import (
	"testing"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
)

func TestXudpReadWrite(t *testing.T) {
	addr, _ := net.ParseDestination("tcp:127.0.0.1:1345")
	mb := make(buf.MultiBuffer, 0, 16)
	m := buf.MultiBufferContainer{
		MultiBuffer: mb,
	}
	var arr [8]byte
	writer := NewPacketWriter(&m, addr, arr)

	source := make(buf.MultiBuffer, 0, 16)
	b := buf.New()
	b.WriteByte('a')
	b.UDP = &addr
	source = append(source, b)
	writer.WriteMultiBuffer(source)

	reader := NewPacketReader(&m)
	dest, err := reader.ReadMultiBuffer()
	common.Must(err)
	if dest[0].Byte(0) != 'a' {
		t.Error("failed to parse xudp buffer")
	}
	if dest[0].UDP.Port != 1345 {
		t.Error("failed to parse xudp buffer")
	}
}
