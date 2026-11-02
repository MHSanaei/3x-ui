package service

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAWGPingEncryptedTunnelWithNativeAccounting(t *testing.T) {
	target := os.Getenv("AWG_ECHO_E2E_TARGET")
	if target == "" {
		t.Skip("set AWG_ECHO_E2E_TARGET inside an isolated test namespace")
	}
	ib, peer, record := seedAWGPing(t)
	db := database.GetDB()
	serverPrivate, serverPublic, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	clientPrivate, clientPublic, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	peer.PublicKey = clientPublic
	var settings amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		t.Fatal(err)
	}
	settings.Clients[0].PublicKey = clientPublic
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(ib).UpdateColumn("settings", string(raw)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(record).UpdateColumn("wg_public_key", clientPublic).Error; err != nil {
		t.Fatal(err)
	}
	if err := saveAWGPing(db, peer.Email, &model.AWGPingPolicy{Enabled: true, Allow: []string{target}}); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.LocalAddr().(*net.UDPAddr).Port
	_ = listener.Close()
	instance := amneziawg.Instance{Id: ib.Id, Tag: ib.Tag, Listen: "127.0.0.1", ListenPort: port, PrivateKey: serverPrivate, PublicKey: serverPublic, Address: []string{"192.0.2.1/24"}, MTU: 1420, Peers: []amneziawg.Peer{{Email: peer.Email, PublicKey: clientPublic, AllowedIPs: []string{"192.0.2.2/32"}}}}
	manager := amneziawgnet.GetManager()
	if err := manager.Ensure(amneziawgnet.Desired{Instance: instance, EchoGate: (&InboundService{}).RecordAWGPing}); err != nil {
		t.Fatal(err)
	}
	defer manager.Remove(ib.Id)
	tun, network, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr("192.0.2.2")}, nil, 1420)
	if err != nil {
		t.Fatal(err)
	}
	client := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	defer client.Close()
	private, err := wireguard.KeyToHex(clientPrivate)
	if err != nil {
		t.Fatal(err)
	}
	public, err := wireguard.KeyToHex(serverPublic)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.IpcSet(fmt.Sprintf("private_key=%s\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n", private, public, port)); err != nil {
		t.Fatal(err)
	}
	if err := client.Up(); err != nil {
		t.Fatal(err)
	}
	ping, err := network.DialPingAddr(netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr(target))
	if err != nil {
		t.Fatal(err)
	}
	defer ping.Close()
	payload := []byte("isolated native accounting probe")
	request := make([]byte, 8+len(payload))
	request[0] = 8
	binary.BigEndian.PutUint16(request[6:8], 17)
	copy(request[8:], payload)
	var checksum uint32
	for i := 0; i+1 < len(request); i += 2 {
		checksum += uint32(binary.BigEndian.Uint16(request[i : i+2]))
	}
	if len(request)%2 != 0 {
		checksum += uint32(request[len(request)-1]) << 8
	}
	for checksum>>16 != 0 {
		checksum = (checksum & 65535) + (checksum >> 16)
	}
	binary.BigEndian.PutUint16(request[2:4], ^uint16(checksum))
	if err := ping.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := ping.Write(request); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 2048)
	n, err := ping.Read(reply)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(request) || reply[0] != 0 || binary.BigEndian.Uint16(reply[6:8]) != 17 || !bytes.Equal(reply[8:n], payload) {
		t.Fatal("incorrect end-to-end echo")
	}
	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", peer.Email).Take(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	want := int64(20 + len(request))
	if traffic.Up != want || traffic.Down != want {
		t.Fatalf("native traffic=%d/%d want=%d/%d", traffic.Up, traffic.Down, want, want)
	}
	if err := saveAWGPing(db, peer.Email, &model.AWGPingPolicy{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	_ = ping.SetDeadline(time.Now().Add(200 * time.Millisecond))
	_, _ = ping.Write(request)
	if _, err := ping.Read(reply); err == nil {
		t.Fatal("revoked permission returned echo")
	}
	var after xray.ClientTraffic
	if err := db.Where("email = ?", peer.Email).Take(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Up != want || after.Down != want {
		t.Fatal("denied packet mutated counters")
	}
}
