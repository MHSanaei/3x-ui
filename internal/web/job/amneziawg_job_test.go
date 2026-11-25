package job

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// A subscribed amneziawg outbound is bridged to the egress listener by tag, so it needs its
// device; one config generation drops (a private endpoint) must not get a device dialing it.
func TestDesiredOutboundInstancesForSubscribedAmneziaWG(t *testing.T) {
	cases := []struct {
		name, endpoint string
		want           []string
	}{
		{"public endpoint gets a device", "8.8.8.8:51820", []string{"p-awg"}},
		{"private endpoint without allowPrivate gets none", "10.0.0.7:51820", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
			priv, pub, err := wgutil.GenerateWireguardKeypair()
			if err != nil {
				t.Fatal(err)
			}
			stored, _ := json.Marshal([]any{map[string]any{
				"tag": "p-awg", "protocol": "amneziawg",
				"settings": map[string]any{
					"secretKey": priv, "address": []string{"10.8.1.2/32"},
					"jc": 4, "jmin": 10, "jmax": 50, "s1": 30, "s2": 40, "h1": "1", "h2": "2", "h3": "3", "h4": "4",
					"peers": []any{map[string]any{"publicKey": pub, "allowedIPs": []string{"0.0.0.0/0"}, "endpoint": c.endpoint}},
				},
			}})
			sub := &model.OutboundSubscription{Url: "https://provider.example/sub", TagPrefix: "p-", Enabled: true, LastFetchedOutbounds: string(stored)}
			if err := database.GetDB().Create(sub).Error; err != nil {
				t.Fatalf("seed subscription: %v", err)
			}
			got, err := NewAmneziaWGJob().desiredOutboundInstances()
			if err != nil {
				t.Fatalf("desiredOutboundInstances: %v", err)
			}
			var tags []string
			for _, d := range got {
				tags = append(tags, d.Instance.Tag)
			}
			if len(tags) != len(c.want) || (len(tags) == 1 && tags[0] != c.want[0]) {
				t.Fatalf("desired outbound tags = %v, want %v", tags, c.want)
			}
		})
	}
}
