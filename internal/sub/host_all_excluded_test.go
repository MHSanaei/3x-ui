package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type formatOutput struct {
	rendered bool
	header   string
	up, down int64
}

func renderFormat(t *testing.T, format string) formatOutput {
	t.Helper()
	switch format {
	case "raw":
		links, _, _, traffic, err := NewSubService("").GetSubs("s1", "req.example.com")
		if err != nil {
			t.Fatalf("GetSubs: %v", err)
		}
		return formatOutput{rendered: len(links) > 0, up: traffic.Up, down: traffic.Down}
	case "json":
		out, header, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("s1", "req.example.com", false)
		if err != nil {
			t.Fatalf("GetJson: %v", err)
		}
		return formatOutput{rendered: strings.TrimSpace(out) != "", header: header}
	default:
		out, header, err := NewSubClashService(false, "", false, NewSubService("")).GetClash("s1", "req.example.com")
		if err != nil {
			t.Fatalf("GetClash: %v", err)
		}
		return formatOutput{rendered: strings.TrimSpace(out) != "", header: header}
	}
}

// An inbound whose every host is excluded from a format must not leak the bare
// inbound into that format, in both host sort modes; usage is still accounted.
func TestSub_AllHostsFormatExcluded_RendersNothing(t *testing.T) {
	for _, mode := range []string{"legacy", "hosts-first"} {
		for _, format := range []string{"raw", "json", "clash"} {
			t.Run(mode+"/"+format, func(t *testing.T) {
				seedSubDB(t)
				setHostSortMode(t, mode)
				ib := seedSubInbound(t, "s1", "allex", 449, 1, tcpTLSStream)
				seedHost(t, &model.Host{
					InboundId: ib.Id, SortOrder: 1, Remark: "A", Address: "a.cdn.com", Port: 443,
					Security: "tls", Sni: "a.sni", ExcludeFromSubTypes: []string{format},
				})
				seedHost(t, &model.Host{
					InboundId: ib.Id, SortOrder: 2, Remark: "B", Address: "b.cdn.com", Port: 443,
					Security: "tls", Sni: "b.sni", ExcludeFromSubTypes: []string{format},
				})
				if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: ib.Id, Email: "allex@e", Up: 300, Down: 400, Enable: true}).Error; err != nil {
					t.Fatalf("seed traffic: %v", err)
				}

				got := renderFormat(t, format)
				if got.rendered {
					t.Fatalf("%s with every host excluded must render nothing", format)
				}
				switch format {
				case "raw":
					if got.up != 300 || got.down != 400 {
						t.Fatalf("usage = up %d/down %d, want 300/400", got.up, got.down)
					}
				default:
					if !strings.HasPrefix(got.header, "upload=300; download=400; ") {
						t.Fatalf("header = %q, want usage prefix", got.header)
					}
				}

				for _, other := range []string{"raw", "json", "clash"} {
					if other != format && !renderFormat(t, other).rendered {
						t.Fatalf("%s is not excluded and must still render", other)
					}
				}
			})
		}
	}
}

// Disabled hosts are switched off, not excluded: an inbound left with only
// disabled hosts keeps rendering its own address, as before.
func TestSub_OnlyDisabledHosts_StillRendersInbound(t *testing.T) {
	for _, mode := range []string{"legacy", "hosts-first"} {
		t.Run(mode, func(t *testing.T) {
			seedSubDB(t)
			setHostSortMode(t, mode)
			ib := seedSubInbound(t, "s1", "dis", 450, 1, tcpTLSStream)
			seedHost(t, &model.Host{
				InboundId: ib.Id, SortOrder: 1, Remark: "D", Address: "d.cdn.com", Port: 443,
				Security: "tls", Sni: "d.sni", IsDisabled: true,
			})
			for _, format := range []string{"raw", "json", "clash"} {
				if !renderFormat(t, format).rendered {
					t.Fatalf("%s: inbound with only disabled hosts must still render", format)
				}
			}
		})
	}
}

// The panel's link export and per-client link paths must also drop an inbound
// whose hosts are all excluded from raw.
func TestLinkProvider_AllHostsRawExcluded_NoLinks(t *testing.T) {
	seedSubDB(t)
	ib := seedSubInbound(t, "s1", "lp", 451, 1, tcpTLSStream)
	seedHost(t, &model.Host{
		InboundId: ib.Id, SortOrder: 1, Remark: "L", Address: "l.cdn.com", Port: 443,
		Security: "tls", Sni: "l.sni", ExcludeFromSubTypes: []string{"raw"},
	})

	provider := NewLinkProvider()
	if links := provider.LinksForClient("req.example.com", ib, "lp@e"); len(links) != 0 {
		t.Fatalf("LinksForClient must be empty, got %v", links)
	}
	if links := provider.LinksForInbounds("req.example.com", []*model.Inbound{ib}); len(links) != 0 {
		t.Fatalf("LinksForInbounds must be empty, got %v", links)
	}
}
