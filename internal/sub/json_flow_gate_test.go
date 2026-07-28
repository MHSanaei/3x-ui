package sub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedFlowInbound(t *testing.T, subId, tag string, port int, stream string) *model.Inbound {
	t.Helper()
	db := database.GetDB()
	uuid := "11111111-2222-4333-8444-" + fmt.Sprintf("%012d", port)
	email := tag + "@e"
	settings := fmt.Sprintf(
		`{"clients":[{"id":%q,"email":%q,"subId":%q,"enable":true,"flow":"xtls-rprx-vision"}],"decryption":"none"}`,
		uuid, email, subId)
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Listen: "203.0.113.5", Port: port,
		Protocol: model.VLESS, Remark: tag, Settings: settings, StreamSettings: stream,
		SubSortIndex: 1,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound %s: %v", tag, err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, UUID: uuid, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
	link := &model.ClientInbound{ClientId: client.Id, InboundId: ib.Id, FlowOverride: "xtls-rprx-vision"}
	if err := db.Create(link).Error; err != nil {
		t.Fatalf("seed client_inbound %s: %v", email, err)
	}
	return ib
}

// A vision flow left on a client after its inbound moved to a transport Vision
// cannot use is stripped from the raw link and the Clash proxy; the JSON
// subscription must agree instead of emitting an outbound xray rejects.
func TestSub_JSONStripsFlowOnUnsupportedTransport(t *testing.T) {
	seedSubDB(t)
	seedFlowInbound(t, "s1", "wsflow", 4601, wsTLSStream)

	links, _, _, _, err := NewSubService("").GetSubs("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if joined := strings.Join(links, "\n"); strings.Contains(joined, "flow=") {
		t.Fatalf("raw link must not carry a flow on ws+tls: %s", joined)
	}

	clash := NewSubClashService(false, "", NewSubService(""))
	yaml, _, err := clash.GetClash("s1", "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if strings.Contains(yaml, "flow:") {
		t.Fatalf("clash proxy must not carry a flow on ws+tls:\n%s", yaml)
	}

	js := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if strings.Contains(out, `"flow"`) {
		t.Fatalf("json outbound must not carry a flow on ws+tls:\n%s", out)
	}
}

// The gate must not strip a flow the transport does support.
func TestSub_JSONKeepsFlowOnTcpTLS(t *testing.T) {
	seedSubDB(t)
	seedFlowInbound(t, "s1", "tcpflow", 4602,
		`{"network":"tcp","security":"tls","tlsSettings":{"serverName":"base.sni"}}`)

	js := NewSubJsonService("", "", "", NewSubService(""))
	out, _, err := js.GetJson("s1", "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(out, `"flow": "xtls-rprx-vision"`) {
		t.Fatalf("json outbound must keep the vision flow on tcp+tls:\n%s", out)
	}
}
