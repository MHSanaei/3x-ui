package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// seedDepletedOnSibling attaches clients d and h to inbounds a and b with d's
// depleted traffic row pointing at b, where AddClientStat's upsert leaves it.
func seedDepletedOnSibling(t *testing.T, proto model.Protocol, port int, settings string) (a, b *model.Inbound) {
	t.Helper()
	setupSettingTestDB(t)
	db := database.GetDB()
	for i, dst := range []**model.Inbound{&a, &b} {
		ib := &model.Inbound{Tag: string(proto) + "-sib-" + string(rune('a'+i)), Enable: true, Port: port + i, Protocol: proto, Settings: settings}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound: %v", err)
		}
		clients, err := (&InboundService{}).GetClients(ib)
		if err != nil {
			t.Fatalf("GetClients: %v", err)
		}
		if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
			t.Fatalf("SyncInbound: %v", err)
		}
		*dst = ib
	}
	rows := []xray.ClientTraffic{
		{InboundId: b.Id, Email: "d", Enable: false, Up: 10, Total: 10},
		{InboundId: b.Id, Email: "h", Enable: true},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}
	return a, b
}

func requireOnlyHealthy(t *testing.T, site string, emails []string) {
	t.Helper()
	if len(emails) != 1 || emails[0] != "h" {
		t.Fatalf("%s serves %v on the sibling inbound, want only [h]: depleted d is still served", site, emails)
	}
}

func TestRuntimeDropsDepletedClientWhoseStatsRowPointsAtSibling(t *testing.T) {
	const vless = `{"clients":[{"email":"d","id":"11111111-1111-1111-1111-11111111111d","enable":true},` +
		`{"email":"h","id":"11111111-1111-1111-1111-11111111111e","enable":true}],"decryption":"none"}`

	t.Run("runtime push", func(t *testing.T) {
		a, _ := seedDepletedOnSibling(t, model.VLESS, 23311, vless)
		built, err := (&InboundService{}).buildInboundForLocalRuntime(database.GetDB(), a)
		if err != nil {
			t.Fatalf("buildInboundForLocalRuntime: %v", err)
		}
		clients, err := (&InboundService{}).GetClients(built)
		if err != nil {
			t.Fatalf("GetClients: %v", err)
		}
		var emails []string
		for _, c := range clients {
			emails = append(emails, c.Email)
		}
		requireOnlyHealthy(t, "buildInboundForLocalRuntime", emails)
	})

	t.Run("mtproto sidecar", func(t *testing.T) {
		a, _ := seedDepletedOnSibling(t, model.MTProto, 23321,
			`{"clients":[{"email":"d","secret":"`+mtprotoTestSecretA+`","enable":true},`+
				`{"email":"h","secret":"`+mtprotoTestSecretB+`","enable":true}]}`)
		instances, err := (&InboundService{}).DesiredMtprotoInstances()
		if err != nil {
			t.Fatalf("DesiredMtprotoInstances: %v", err)
		}
		for _, inst := range instances {
			if inst.Id != a.Id {
				continue
			}
			var emails []string
			for _, sec := range inst.Secrets {
				emails = append(emails, sec.Name)
			}
			requireOnlyHealthy(t, "DesiredMtprotoInstances", emails)
			return
		}
		t.Fatal("sibling mtproto inbound missing from desired instances")
	})

	t.Run("tuic sidecar", func(t *testing.T) {
		a, _ := seedDepletedOnSibling(t, model.TUIC, 23331,
			`{"certificate":"/c.pem","private_key":"/k.pem","clients":[`+
				`{"id":"11111111-1111-1111-1111-11111111111d","password":"pd","email":"d","enable":true},`+
				`{"id":"11111111-1111-1111-1111-11111111111e","password":"ph","email":"h","enable":true}]}`)
		instances, err := (&InboundService{}).DesiredTuicInstances()
		if err != nil {
			t.Fatalf("DesiredTuicInstances: %v", err)
		}
		for _, inst := range instances {
			if inst.Id != a.Id {
				continue
			}
			var emails []string
			for _, c := range inst.Clients {
				emails = append(emails, c.Email)
			}
			requireOnlyHealthy(t, "DesiredTuicInstances", emails)
			return
		}
		t.Fatal("sibling tuic inbound missing from desired instances")
	})

	t.Run("amneziawg interface", func(t *testing.T) {
		settings, err := json.Marshal(amneziawg.InboundSettings{
			Server: &amneziawg.ServerSettings{SubnetIP: "10.8.1.0", SubnetCIDR: 24},
			Clients: []model.Client{
				{Email: "d", Enable: true, PublicKey: "pk-d", AllowedIPs: []string{"10.8.1.2/32"}},
				{Email: "h", Enable: true, PublicKey: "pk-h", AllowedIPs: []string{"10.8.1.3/32"}},
			},
		})
		if err != nil {
			t.Fatalf("marshal awg settings: %v", err)
		}
		a, _ := seedDepletedOnSibling(t, model.AmneziaWG, 23341, string(settings))
		instances, err := (&InboundService{}).DesiredAmneziaWGInstances()
		if err != nil {
			t.Fatalf("DesiredAmneziaWGInstances: %v", err)
		}
		for _, inst := range instances {
			if inst.Id != a.Id {
				continue
			}
			var emails []string
			for _, p := range inst.Peers {
				emails = append(emails, p.Email)
			}
			requireOnlyHealthy(t, "DesiredAmneziaWGInstances", emails)
			return
		}
		t.Fatal("sibling amneziawg inbound missing from desired instances")
	})
}
