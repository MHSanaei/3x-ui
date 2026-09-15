package xray

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

func hotConfigWithClients(clients string) *Config {
	cfg := makeHotConfig()
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag == "inbound-1080" {
			cfg.InboundConfigs[i].Settings = json_util.RawMessage(`{"clients":` + clients + `}`)
		}
	}
	return cfg
}

// diffInboundUsers refuses shadowsocks and hysteria, so their dropped clients
// reach the guard through the inbound instead of through a per-user op.
func TestHotDiffDropsUsersOnProtocolsItCannotDiff(t *testing.T) {
	for _, protocol := range []string{"shadowsocks", "hysteria"} {
		t.Run(protocol, func(t *testing.T) {
			withClients := func(clients string) *Config {
				cfg := makeHotConfig()
				ib := &cfg.InboundConfigs[1]
				ib.Protocol = protocol
				ib.Settings = json_util.RawMessage(`{"clients":` + clients + `}`)
				return cfg
			}
			both := `[{"email":"a@x","password":"pa"},{"email":"b@x","password":"pb"}]`
			onlyA := `[{"email":"a@x","password":"pa"}]`

			diff, ok := ComputeHotDiff(withClients(both), withClients(onlyA))
			if !ok {
				t.Fatal("a dropped client must stay API-applicable")
			}
			if !diff.DropsUsers() {
				t.Fatalf("DropsUsers = false for a dropped %s client (removed=%+v added=%+v dropped=%+v)",
					protocol, diff.RemovedUsers, diff.AddedUsers, diff.DroppedClients)
			}

			edited, ok := ComputeHotDiff(withClients(both), withClients(`[{"email":"a@x","password":"pa"},{"email":"b@x","password":"pb","level":1}]`))
			if !ok {
				t.Fatal("a client edit must stay API-applicable")
			}
			if edited.DropsUsers() {
				t.Fatal("an edited client is still served")
			}
		})
	}
}

// A disable or a delete takes the client out of the generated config; an edit
// keeps the email and re-adds it. Only the first leaves sessions running.
func TestHotDiffDropsUsers(t *testing.T) {
	cases := []struct {
		name string
		old  string
		new  string
		want bool
	}{
		{
			"client taken out of the config",
			`[{"email":"a@x","id":"11111111-1111-1111-1111-111111111111","enable":true}]`,
			`[]`,
			true,
		},
		{
			"edited in place",
			`[{"email":"a@x","id":"11111111-1111-1111-1111-111111111111","enable":true}]`,
			`[{"email":"a@x","id":"11111111-1111-1111-1111-111111111111","limitIp":5,"enable":true}]`,
			false,
		},
		{
			"added",
			`[]`,
			`[{"email":"a@x","id":"11111111-1111-1111-1111-111111111111","enable":true}]`,
			false,
		},
		{
			"renamed",
			`[{"email":"a@x","id":"11111111-1111-1111-1111-111111111111","enable":true}]`,
			`[{"email":"b@x","id":"11111111-1111-1111-1111-111111111111","enable":true}]`,
			true,
		},
		{
			"one dropped, one edited",
			`[{"email":"a@x","id":"11111111-1111-1111-1111-111111111111","enable":true},{"email":"b@x","id":"22222222-2222-2222-2222-222222222222","enable":true}]`,
			`[{"email":"b@x","id":"22222222-2222-2222-2222-222222222222","limitIp":5,"enable":true}]`,
			true,
		},
		{
			"unchanged",
			`[{"email":"a@x","id":"11111111-1111-1111-1111-111111111111","enable":true}]`,
			`[{"email":"a@x","id":"11111111-1111-1111-1111-111111111111","enable":true}]`,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff, ok := ComputeHotDiff(hotConfigWithClients(tc.old), hotConfigWithClients(tc.new))
			if !ok {
				t.Fatalf("diff of %s -> %s must be API-applicable", tc.old, tc.new)
			}
			if got := diff.DropsUsers(); got != tc.want {
				t.Fatalf("DropsUsers = %v, want %v (removed=%+v added=%+v)",
					got, tc.want, diff.RemovedUsers, diff.AddedUsers)
			}
		})
	}
}
