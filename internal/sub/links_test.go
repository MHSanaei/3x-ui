package sub

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSplitLinkLines(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single_line", "vless://abc", []string{"vless://abc"}},
		{"two_lines", "vless://abc\nvmess://xyz", []string{"vless://abc", "vmess://xyz"}},
		{"trims_each_line", "  vless://abc  \n\tvmess://xyz\t", []string{"vless://abc", "vmess://xyz"}},
		{"skips_blank_lines", "vless://abc\n\n\nvmess://xyz\n", []string{"vless://abc", "vmess://xyz"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitLinkLines(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("splitLinkLines(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}

func TestSplitLinkLines_EmptyInputIsNil(t *testing.T) {
	if got := splitLinkLines(""); got != nil {
		t.Fatalf("splitLinkLines(\"\") = %#v, want nil", got)
	}
}

func TestSplitLinkLines_WhitespaceOnlyHasNoEntries(t *testing.T) {
	got := splitLinkLines("   \n\t  \n")
	if len(got) != 0 {
		t.Fatalf("splitLinkLines(whitespace) = %#v, want empty slice", got)
	}
}

func TestLinksForClient_UsesHostEndpoints(t *testing.T) {
	seedSubDB(t)
	inbound := seedSubInbound(t, "s-gate", "gate", 4431, 1, `{"network":"tcp","security":"none"}`)
	seedHost(t, &model.Host{
		InboundId: inbound.Id, Remark: "public", Address: "proxy.example.com",
		Port: 443, Security: "same",
	})

	links := NewLinkProvider().LinksForClient("req.example.com", inbound, "gate@e")

	if len(links) != 1 {
		t.Fatalf("links = %d, want 1: %v", len(links), links)
	}
	if !strings.Contains(links[0], "proxy.example.com:443") {
		t.Fatalf("link = %q, want the host endpoint proxy.example.com:443", links[0])
	}
}

// LinksForClient (per-client QR / links API) must use the clients-table UUID
// when the inbound settings JSON still embeds a stale id — same source as
// /inbounds/list and allLinks (#6436).
func TestLinksForClient_UsesClientsTableUUIDWhenSettingsStale(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	stale := "11111111-1111-1111-1111-111111111111"
	fresh := "22222222-2222-2222-2222-222222222222"
	settings := `{"clients":[{"id":"` + stale + `","email":"stale@e","subId":"subStale","enable":true}],"decryption":"none"}`
	ib := &model.Inbound{
		UserId: 1, Tag: "stale-uuid-qr", Enable: true, Listen: "203.0.113.5", Port: 4433,
		Protocol: model.VLESS, Remark: "StaleQR", Settings: settings,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: "stale@e", SubID: "subStale", UUID: fresh, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}

	links := NewLinkProvider().LinksForClient("req.example.com", ib, "stale@e")
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1: %v", len(links), links)
	}
	if !strings.Contains(links[0], fresh) {
		t.Fatalf("link missing fresh UUID %q: %s", fresh, links[0])
	}
	if strings.Contains(links[0], stale) {
		t.Fatalf("link still carries stale settings UUID %q: %s", stale, links[0])
	}
}
