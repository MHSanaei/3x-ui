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

// The panel answers share-link requests as soon as it serves, so the provider
// has to be installed before the server exists: a link request that arrived
// earlier got "sub link provider not registered".
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
