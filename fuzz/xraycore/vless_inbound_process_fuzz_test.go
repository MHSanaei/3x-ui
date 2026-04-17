package xraycorefuzz

import (
	"bytes"
	"context"
	"errors"
	"io"
	stdnet "net"
	"sync"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	xuuid "github.com/xtls/xray-core/common/uuid"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/routing"
	xconf "github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/proxy/vless"
	vlessencoding "github.com/xtls/xray-core/proxy/vless/encoding"
	vinbound "github.com/xtls/xray-core/proxy/vless/inbound"
	"github.com/xtls/xray-core/transport"
)

const maxInboundProcessIteration = 1500 * time.Millisecond

func FuzzXrayCoreVLESSInboundProcessPreAuth(f *testing.F) {
	for _, seed := range stage2VLESSProcessSeeds(f) {
		f.Add(seed)
	}
	handler := mustStage2PlainVLESSHandler(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		if tooLarge(data, maxFirstPacketBytes) {
			return
		}
		start := time.Now()
		dispatcher := newRecordingDispatcher()
		conn := newVLESSFuzzConn(data)
		err := handler.Process(vlessProcessContext(), xnet.Network_TCP, conn, dispatcher)

		expectedDispatch := shouldVLESSProcessDispatch(t, data, false)
		assertVLESSProcessOutcome(t, expectedDispatch, dispatcher, err)
		failIfSlow(t, start, maxInboundProcessIteration)
	})
}

func FuzzXrayCoreVLESSInboundFallbackPreAuth(f *testing.F) {
	for _, seed := range stage2VLESSFallbackSeeds(f) {
		f.Add(seed)
	}
	handler := mustStage2FallbackVLESSHandler(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		if tooLarge(data, maxFirstPacketBytes) {
			return
		}
		start := time.Now()
		dispatcher := newRecordingDispatcher()
		conn := newVLESSFuzzConn(data)
		err := handler.Process(vlessProcessContext(), xnet.Network_TCP, conn, dispatcher)

		expectedDispatch := shouldVLESSProcessDispatch(t, data, true)
		assertVLESSProcessOutcome(t, expectedDispatch, dispatcher, err)
		failIfSlow(t, start, maxInboundProcessIteration)
	})
}

func TestXrayCoreVLESSInboundProcessValidSeedsDispatch(t *testing.T) {
	handler := mustStage2PlainVLESSHandler(t)
	for i, seed := range stage2ExpectedDispatchSeeds(t) {
		dispatcher := newRecordingDispatcher()
		err := handler.Process(vlessProcessContext(), xnet.Network_TCP, newVLESSFuzzConn(seed), dispatcher)
		if err != nil {
			t.Fatalf("valid seed %d was rejected: %v", i, err)
		}
		if !dispatcher.called {
			t.Fatalf("valid seed %d did not reach DispatchLink", i)
		}
	}
}

func TestXrayCoreVLESSInboundProcessRejectsMalformedSeeds(t *testing.T) {
	handler := mustStage2PlainVLESSHandler(t)
	for i, seed := range stage2ExpectedRejectSeeds(t) {
		dispatcher := newRecordingDispatcher()
		err := handler.Process(vlessProcessContext(), xnet.Network_TCP, newVLESSFuzzConn(seed), dispatcher)
		if err == nil {
			t.Fatalf("malformed seed %d returned nil error", i)
		}
		if dispatcher.called {
			t.Fatalf("malformed seed %d reached DispatchLink", i)
		}
	}
}

func TestXrayCoreVLESSTrailingGarbageDoesNotChangeHeader(t *testing.T) {
	valid := mustValidVLESSFirstPacket(t)
	withGarbage := append(append([]byte(nil), valid...), []byte("arbitrary trailing body bytes")...)

	left := mustDecodeVLESSHeader(t, valid, false)
	right := mustDecodeVLESSHeader(t, withGarbage, false)
	if left.destination != right.destination || left.command != right.command || left.flow != right.flow || left.userID != right.userID {
		t.Fatalf("trailing bytes changed parsed header: %#v != %#v", left, right)
	}
}

func shouldVLESSProcessDispatch(t testing.TB, data []byte, firstBufferMode bool) bool {
	t.Helper()
	decoded, ok := decodeVLESSHeaderForOracle(t, data, firstBufferMode)
	if !ok {
		return false
	}
	switch decoded.flow {
	case "":
	case vless.XRV:
		return false
	default:
		return false
	}
	switch decoded.command {
	case protocol.RequestCommandTCP, protocol.RequestCommandUDP, protocol.RequestCommandMux:
		return true
	case protocol.RequestCommandRvs:
		return false
	default:
		return false
	}
}

type decodedVLESSHeader struct {
	userID      string
	command     protocol.RequestCommand
	destination string
	flow        string
}

func mustDecodeVLESSHeader(t testing.TB, data []byte, firstBufferMode bool) decodedVLESSHeader {
	t.Helper()
	decoded, ok := decodeVLESSHeaderForOracle(t, data, firstBufferMode)
	if !ok {
		t.Fatalf("failed to decode VLESS header from valid seed")
	}
	return decoded
}

func decodeVLESSHeaderForOracle(t testing.TB, data []byte, firstBufferMode bool) (decodedVLESSHeader, bool) {
	t.Helper()
	validator := mustVLESSValidator(t)
	var (
		request *protocol.RequestHeader
		addons  *vlessencoding.Addons
		err     error
	)
	if firstBufferMode {
		if len(data) < 18 {
			return decodedVLESSHeader{}, false
		}
		first := buf.FromBytes(append([]byte(nil), data...))
		reader := &buf.BufferedReader{
			Reader: buf.NewReader(bytes.NewReader(nil)),
			Buffer: buf.MultiBuffer{first},
		}
		_, request, addons, _, err = vlessencoding.DecodeRequestHeader(true, first, reader, validator)
	} else {
		_, request, addons, _, err = vlessencoding.DecodeRequestHeader(false, nil, bytes.NewReader(data), validator)
	}
	if err != nil || request == nil || request.User == nil || request.User.Account == nil || addons == nil {
		return decodedVLESSHeader{}, false
	}
	account, ok := request.User.Account.(*vless.MemoryAccount)
	if !ok || !account.ID.Equals(mustVLESSID(t)) || request.Address == nil {
		return decodedVLESSHeader{}, false
	}
	return decodedVLESSHeader{
		userID:      account.ID.String(),
		command:     request.Command,
		destination: request.Destination().String(),
		flow:        addons.Flow,
	}, true
}

func assertVLESSProcessOutcome(t *testing.T, expectedDispatch bool, dispatcher *recordingDispatcher, err error) {
	t.Helper()
	if expectedDispatch {
		if err != nil {
			t.Fatalf("valid first packet was rejected before dispatch: %v", err)
		}
		if !dispatcher.called {
			t.Fatal("valid first packet did not reach DispatchLink")
		}
		return
	}
	if dispatcher.called {
		t.Fatalf("malformed or unauthorized first packet reached DispatchLink for %s", dispatcher.dest.String())
	}
	if err == nil {
		t.Fatal("malformed or unauthorized first packet returned nil error without dispatch")
	}
}

func stage2VLESSProcessSeeds(t testing.TB) [][]byte {
	t.Helper()
	seeds := append([][]byte{}, validVLESSFirstPacketSeeds(t)...)
	seeds = append(seeds, mutateVLESSPacketSeeds(mustValidVLESSFirstPacket(t))...)
	seeds = append(seeds,
		vlessPacketWithRawAddons(t, bytes.Repeat([]byte{0x41}, 255), protocol.RequestCommandTCP, xnet.DomainAddress("example.com"), xnet.Port(443)),
		vlessPacketWithRawAddons(t, []byte{0x0a, 0x03, 'b', 'a', 'd'}, protocol.RequestCommandUDP, xnet.ParseAddress("8.8.8.8"), xnet.Port(53)),
		append(mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.ParseAddress("127.0.0.1"), xnet.Port(80), &vlessencoding.Addons{}), 0, 1, 2, 3, 4),
	)
	return seeds
}

func stage2VLESSFallbackSeeds(t testing.TB) [][]byte {
	t.Helper()
	seeds := stage2VLESSProcessSeeds(t)
	seeds = append(seeds,
		[]byte("GET /stage2 HTTP/1.1\r\nHost: example.com\r\n\r\n"),
		[]byte("POST /missing HTTP/1.1\r\nHost: example.com\r\nContent-Length: 4\r\n\r\nbody"),
		[]byte("* HTTP/2.0\r\n\r\n"),
	)
	return seeds
}

func stage2ExpectedDispatchSeeds(t testing.TB) [][]byte {
	t.Helper()
	return [][]byte{
		mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.DomainAddress("example.com"), xnet.Port(443), &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.ParseAddress("127.0.0.1"), xnet.Port(80), &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.ParseAddress("::1"), xnet.Port(443), &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandUDP, xnet.ParseAddress("8.8.8.8"), xnet.Port(53), &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandMux, nil, 0, &vlessencoding.Addons{}),
	}
}

func stage2ExpectedRejectSeeds(t testing.TB) [][]byte {
	t.Helper()
	valid := mustValidVLESSFirstPacket(t)
	return [][]byte{
		{},
		{0},
		append([]byte(nil), valid[:17]...),
		mutateVLESSPacketSeeds(valid)[7],
		mutateVLESSPacketSeeds(valid)[8],
		mutateVLESSPacketSeeds(valid)[9],
		vlessPacketWithRawAddons(t, []byte{0x0a, 0x03, 'b', 'a', 'd'}, protocol.RequestCommandTCP, xnet.DomainAddress("example.com"), xnet.Port(443)),
		mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.DomainAddress("example.com"), xnet.Port(443), &vlessencoding.Addons{Flow: vless.XRV}),
		mustVLESSFirstPacket(t, protocol.RequestCommandRvs, nil, 0, &vlessencoding.Addons{}),
	}
}

var (
	stage2HandlersOnce sync.Once
	stage2PlainHandler *vinbound.Handler
	stage2FBHandler    *vinbound.Handler
	stage2HandlersErr  error
)

func mustStage2PlainVLESSHandler(t testing.TB) *vinbound.Handler {
	t.Helper()
	mustInitStage2Handlers(t)
	return stage2PlainHandler
}

func mustStage2FallbackVLESSHandler(t testing.TB) *vinbound.Handler {
	t.Helper()
	mustInitStage2Handlers(t)
	return stage2FBHandler
}

func mustInitStage2Handlers(t testing.TB) {
	t.Helper()
	stage2HandlersOnce.Do(func() {
		stage2PlainHandler, stage2HandlersErr = newStage2VLESSHandler(nil)
		if stage2HandlersErr != nil {
			return
		}
		stage2FBHandler, stage2HandlersErr = newStage2VLESSHandler([]*vinbound.Fallback{
			{Path: "stage2-unreachable-fallback-target", Type: "stage2-invalid-network", Dest: "unused"},
		})
	})
	if stage2HandlersErr != nil {
		t.Fatalf("failed to initialize stage2 VLESS handler: %v", stage2HandlersErr)
	}
}

func newStage2VLESSHandler(fallbacks []*vinbound.Fallback) (*vinbound.Handler, error) {
	pbConfig, err := (&xconf.Config{
		OutboundConfigs: []xconf.OutboundDetourConfig{{Protocol: "freedom", Tag: "direct"}},
	}).Build()
	if err != nil {
		return nil, err
	}
	instance, err := core.New(pbConfig)
	if err != nil {
		return nil, err
	}
	user := protocol.ToProtoUser(&protocol.MemoryUser{
		Account: &vless.MemoryAccount{
			ID:         protocol.NewID(mustVLESSUUID()),
			Encryption: "none",
		},
		Email: "seed@example",
	})
	rawHandler, err := core.CreateObject(instance, &vinbound.Config{
		Clients:    []*protocol.User{user},
		Fallbacks:  fallbacks,
		Decryption: "none",
	})
	if err != nil {
		return nil, err
	}
	handler, ok := rawHandler.(*vinbound.Handler)
	if !ok {
		return nil, errors.New("VLESS inbound config did not create *inbound.Handler")
	}
	return handler, nil
}

func mustVLESSUUID() xuuid.UUID {
	id, err := xuuid.ParseString(seedVLESSUUID)
	if err != nil {
		panic(err)
	}
	return id
}

func vlessProcessContext() context.Context {
	return session.ContextWithInbound(context.Background(), &session.Inbound{
		Source: xnet.TCPDestination(xnet.ParseAddress("203.0.113.10"), xnet.Port(50000)),
		Local:  xnet.TCPDestination(xnet.ParseAddress("127.0.0.1"), xnet.Port(443)),
		Tag:    "stage2-vless",
	})
}

type recordingDispatcher struct {
	called bool
	dest   xnet.Destination
}

func newRecordingDispatcher() *recordingDispatcher {
	return &recordingDispatcher{}
}

func (*recordingDispatcher) Type() interface{} { return routing.DispatcherType() }
func (*recordingDispatcher) Start() error      { return nil }
func (*recordingDispatcher) Close() error      { return nil }

func (d *recordingDispatcher) Dispatch(context.Context, xnet.Destination) (*transport.Link, error) {
	return nil, errors.New("Dispatch should not be used by VLESS Process harness")
}

func (d *recordingDispatcher) DispatchLink(_ context.Context, dest xnet.Destination, _ *transport.Link) error {
	d.called = true
	d.dest = dest
	return nil
}

type vlessFuzzConn struct {
	reader *bytes.Reader
	writes bytes.Buffer
}

func newVLESSFuzzConn(data []byte) *vlessFuzzConn {
	return &vlessFuzzConn{reader: bytes.NewReader(append([]byte(nil), data...))}
}

func (c *vlessFuzzConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *vlessFuzzConn) Write(p []byte) (int, error) {
	return c.writes.Write(p)
}

func (*vlessFuzzConn) Close() error {
	return nil
}

func (*vlessFuzzConn) LocalAddr() stdnet.Addr {
	return &stdnet.TCPAddr{IP: stdnet.ParseIP("127.0.0.1"), Port: 443}
}

func (*vlessFuzzConn) RemoteAddr() stdnet.Addr {
	return &stdnet.TCPAddr{IP: stdnet.ParseIP("203.0.113.10"), Port: 50000}
}

func (*vlessFuzzConn) SetDeadline(time.Time) error {
	return nil
}

func (*vlessFuzzConn) SetReadDeadline(time.Time) error {
	return nil
}

func (*vlessFuzzConn) SetWriteDeadline(time.Time) error {
	return nil
}

var _ routing.Dispatcher = (*recordingDispatcher)(nil)
var _ interface {
	io.Reader
	io.Writer
} = (*vlessFuzzConn)(nil)
