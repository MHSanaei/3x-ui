package sub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// seedSubProtocolInbound seeds an inbound of the given protocol with one client
// wired into the clients/client_inbounds tables so getInboundsBySubId resolves it.
func seedSubProtocolInbound(t *testing.T, subId, tag string, port, subSortIndex int, stream string, protocol model.Protocol) *model.Inbound {
	t.Helper()
	db := database.GetDB()
	uuid := "11111111-2222-4333-8444-" + fmt.Sprintf("%012d", port)
	email := tag + "@e"
	settings := fmt.Sprintf(`{"clients":[{"id":%q,"email":%q,"subId":%q,"enable":true}]}`, uuid, email, subId)
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Listen: "203.0.113.5", Port: port,
		Protocol: protocol, Remark: tag, Settings: settings, StreamSettings: stream,
		SubSortIndex: subSortIndex,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound %s: %v", tag, err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, UUID: uuid, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound %s: %v", email, err)
	}
	return ib
}

// The member tag suffix is the inbound's real protocol, not its transport
// network: a vmess/tcp member is tagged bal-N-vmess, not the old bal-N-vless.
func TestSubJson_BalancerMemberTagUsesProtocol(t *testing.T) {
	seedSubDB(t)
	vm := seedSubProtocolInbound(t, "s1", "vm", 4901, 1, `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`, model.VMESS)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "proto", Strategy: "random", InboundIds: []int{vm.Id}, SortOrder: 1, Enabled: true,
	})

	js := NewSubJsonService("", "", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	balancerDoc := findDocByRemarks(parseSubJsonDocs(t, out), "proto")
	if balancerDoc == nil {
		t.Fatalf("balancer doc missing:\n%s", out)
	}
	tags := docOutboundTags(balancerDoc)
	if !strings.Contains(strings.Join(tags, ","), "bal-1-vmess") {
		t.Fatalf("vmess member tag = %v, want a bal-1-vmess suffix", tags)
	}
}
