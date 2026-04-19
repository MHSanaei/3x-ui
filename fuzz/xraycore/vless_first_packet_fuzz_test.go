package xraycorefuzz

import (
	"bytes"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/proxy/vless"
	vlessencoding "github.com/xtls/xray-core/proxy/vless/encoding"
)

const (
	seedVLESSUUID       = "11111111-1111-1111-1111-111111111111"
	maxFirstPacketBytes = 16 << 10
	maxPacketIteration  = time.Second
)

func FuzzXrayCoreVLESSFirstPacket(f *testing.F) {
	validSeeds := validVLESSFirstPacketSeeds(f)
	for _, seed := range validSeeds {
		f.Add(seed)
	}
	for _, seed := range mutateVLESSPacketSeeds(validSeeds[0]) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if tooLarge(data, maxFirstPacketBytes) {
			return
		}
		start := time.Now()
		validator := mustVLESSValidator(t)

		checkDecodedVLESSRequest(t, data, validator, false)
		if len(data) >= 18 {
			checkDecodedVLESSRequest(t, data, validator, true)
		}

		failIfSlow(t, start, maxPacketIteration)
	})
}

func TestXrayCoreVLESSWrongUUIDRejected(t *testing.T) {
	valid := mustValidVLESSFirstPacket(t)
	wrongUUID := append([]byte(nil), valid...)
	wrongUUID[1] ^= 0x80

	validator := mustVLESSValidator(t)
	userSentID, request, _, _, err := vlessencoding.DecodeRequestHeader(false, nil, bytes.NewReader(wrongUUID), validator)
	if err == nil || request != nil || userSentID != nil {
		t.Fatalf("wrong UUID packet authenticated: userSentID=%x request=%#v err=%v", userSentID, request, err)
	}
}

func checkDecodedVLESSRequest(t *testing.T, data []byte, validator *vless.MemoryValidator, isFirstBuffer bool) {
	t.Helper()

	var (
		userSentID []byte
		request    *protocol.RequestHeader
		err        error
	)
	if isFirstBuffer {
		first := buf.FromBytes(append([]byte(nil), data...))
		reader := &buf.BufferedReader{
			Reader: buf.NewReader(bytes.NewReader(nil)),
			Buffer: buf.MultiBuffer{first},
		}
		userSentID, request, _, _, err = vlessencoding.DecodeRequestHeader(true, first, reader, validator)
	} else {
		userSentID, request, _, _, err = vlessencoding.DecodeRequestHeader(false, nil, bytes.NewReader(data), validator)
	}
	if err != nil {
		return
	}
	if request == nil {
		t.Fatal("DecodeRequestHeader returned nil request without error")
	}
	if request.User == nil || request.User.Account == nil {
		t.Fatal("DecodeRequestHeader accepted packet without authenticated user")
	}
	account, ok := request.User.Account.(*vless.MemoryAccount)
	if !ok {
		t.Fatalf("DecodeRequestHeader returned unexpected account type %T", request.User.Account)
	}
	validID := mustVLESSID(t)
	if !account.ID.Equals(validID) {
		t.Fatalf("DecodeRequestHeader authenticated unexpected account %s", account.ID.String())
	}
	if len(userSentID) != 16 {
		t.Fatalf("DecodeRequestHeader returned malformed user id length %d", len(userSentID))
	}
	switch request.Command {
	case protocol.RequestCommandTCP, protocol.RequestCommandUDP, protocol.RequestCommandMux, protocol.RequestCommandRvs:
	default:
		t.Fatalf("DecodeRequestHeader accepted invalid command 0x%x", byte(request.Command))
	}
	if request.Address == nil {
		t.Fatal("DecodeRequestHeader accepted request without destination address")
	}
}

func mustVLESSValidator(t testing.TB) *vless.MemoryValidator {
	t.Helper()
	validator := new(vless.MemoryValidator)
	user := &protocol.MemoryUser{
		Account: &vless.MemoryAccount{
			ID:         mustVLESSID(t),
			Encryption: "none",
		},
		Email: "seed@example",
	}
	if err := validator.Add(user); err != nil {
		t.Fatalf("failed to add VLESS seed user: %v", err)
	}
	return validator
}

func mustVLESSID(t testing.TB) *protocol.ID {
	t.Helper()
	id, err := uuid.ParseString(seedVLESSUUID)
	if err != nil {
		t.Fatalf("invalid seed UUID: %v", err)
	}
	return protocol.NewID(id)
}

func mustValidVLESSFirstPacket(t testing.TB) []byte {
	t.Helper()
	return mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.DomainAddress("example.com"), xnet.Port(443), &vlessencoding.Addons{})
}

func validVLESSFirstPacketSeeds(t testing.TB) [][]byte {
	t.Helper()
	return [][]byte{
		mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.DomainAddress("example.com"), xnet.Port(443), &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.ParseAddress("127.0.0.1"), xnet.Port(80), &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.ParseAddress("::1"), xnet.Port(443), &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandUDP, xnet.ParseAddress("8.8.8.8"), xnet.Port(53), &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandMux, nil, 0, &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandRvs, nil, 0, &vlessencoding.Addons{}),
		mustVLESSFirstPacket(t, protocol.RequestCommandTCP, xnet.DomainAddress("example.com"), xnet.Port(443), &vlessencoding.Addons{Flow: vless.XRV}),
		vlessPacketWithRawAddons(t, []byte{0x0a, 0x03, 'b', 'a', 'd'}, protocol.RequestCommandTCP, xnet.DomainAddress("example.com"), xnet.Port(443)),
	}
}

func mustVLESSFirstPacket(t testing.TB, command protocol.RequestCommand, address xnet.Address, port xnet.Port, addons *vlessencoding.Addons) []byte {
	t.Helper()
	user := &protocol.MemoryUser{
		Account: &vless.MemoryAccount{
			ID:         mustVLESSID(t),
			Encryption: "none",
		},
		Email: "seed@example",
	}
	request := &protocol.RequestHeader{
		Version: vlessencoding.Version,
		Command: command,
		Address: address,
		Port:    port,
		User:    user,
	}
	var out bytes.Buffer
	if err := vlessencoding.EncodeRequestHeader(&out, request, addons); err != nil {
		t.Fatalf("failed to build seed VLESS first packet: %v", err)
	}
	return out.Bytes()
}

func vlessPacketWithRawAddons(t testing.TB, addons []byte, command protocol.RequestCommand, address xnet.Address, port xnet.Port) []byte {
	t.Helper()
	if len(addons) > 255 {
		t.Fatalf("raw addons too large: %d", len(addons))
	}
	base := mustVLESSFirstPacket(t, command, address, port, &vlessencoding.Addons{})
	out := make([]byte, 0, len(base)+len(addons))
	out = append(out, base[:17]...)
	out = append(out, byte(len(addons)))
	out = append(out, addons...)
	out = append(out, base[18:]...)
	return out
}

func mutateVLESSPacketSeeds(valid []byte) [][]byte {
	seeds := [][]byte{
		{},
		{0x00},
		{0x00, 0x11},
		bytes.Repeat([]byte{0xff}, 32),
		append([]byte(nil), valid[:1]...),
		append([]byte(nil), valid[:17]...),
		append([]byte(nil), valid[:18]...),
	}

	wrongUUID := append([]byte(nil), valid...)
	wrongUUID[1] ^= 0x80
	seeds = append(seeds, wrongUUID)

	badVersion := append([]byte(nil), valid...)
	badVersion[0] = 0xff
	seeds = append(seeds, badVersion)

	badCommand := append([]byte(nil), valid...)
	if len(badCommand) > 18 {
		badCommand[18] = 0xff
	}
	seeds = append(seeds, badCommand)

	badAddressType := append([]byte(nil), valid...)
	if len(badAddressType) > 21 {
		badAddressType[21] = 0xff
	}
	seeds = append(seeds, badAddressType)

	badDomainLength := append([]byte(nil), valid...)
	if len(badDomainLength) > 22 {
		badDomainLength[22] = 0xff
	}
	seeds = append(seeds, badDomainLength)

	oversizedAddons := append([]byte(nil), valid[:17]...)
	oversizedAddons = append(oversizedAddons, 0xff, 0x01, 0x02, 0x03)
	seeds = append(seeds, oversizedAddons)

	validWithGarbage := append([]byte(nil), valid...)
	validWithGarbage = append(validWithGarbage, []byte("GET /garbage HTTP/1.1\r\nHost: fuzz\r\n\r\n")...)
	seeds = append(seeds, validWithGarbage)

	badIPv6 := append([]byte(nil), valid[:18]...)
	badIPv6 = append(badIPv6, byte(protocol.RequestCommandTCP), 0x01, 0xbb, byte(protocol.AddressTypeIPv6), 0x00, 0x01)
	seeds = append(seeds, badIPv6)

	return seeds
}
