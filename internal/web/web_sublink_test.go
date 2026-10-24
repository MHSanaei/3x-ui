package web

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type fakeSubLinkProvider struct {
	links []string
}

func (f fakeSubLinkProvider) SubLinksForSubId(host, subId string) ([]string, error) {
	return f.links, nil
}

func (f fakeSubLinkProvider) LinksForClient(host string, inbound *model.Inbound, email string) []string {
	return f.links
}

func (f fakeSubLinkProvider) LinksForInbounds(host string, inbounds []*model.Inbound) []string {
	return f.links
}

// A link request can arrive as soon as the panel serves, so NewServer must install the provider.
func TestNewServerInstallsTheShareLinkProviderBeforeServing(t *testing.T) {
	service.RegisterSubLinkProvider(nil)
	t.Cleanup(func() { service.RegisterSubLinkProvider(nil) })

	NewServer(fakeSubLinkProvider{links: []string{"vless://example"}})

	links, err := (&service.InboundService{}).GetSubLinks("panel.example", "subid")
	if err != nil {
		t.Fatalf("the share-link provider must be installed before the panel serves: %v", err)
	}
	if len(links) != 1 || links[0] != "vless://example" {
		t.Fatalf("links = %v, want the provider's links", links)
	}
}
