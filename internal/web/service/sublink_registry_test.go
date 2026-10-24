package service

import (
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

// Startup writes the provider registry while link requests read it; under -race an
// unguarded registry fails here.
func TestSubLinkProviderRegistrationIsRaceFree(t *testing.T) {
	t.Cleanup(func() { RegisterSubLinkProvider(nil) })

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			RegisterSubLinkProvider(fakeSubLinkProvider{links: []string{"vless://example"}})
		}()
		go func() {
			defer wg.Done()
			_, _ = (&InboundService{}).GetSubLinks("panel.example", "subid")
		}()
	}
	wg.Wait()
}
