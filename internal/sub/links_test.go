package sub

import (
	"reflect"
	"strings"
	"testing"

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
