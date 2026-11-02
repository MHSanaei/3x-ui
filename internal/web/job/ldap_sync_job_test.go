package job

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func initLdapJobDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
}

func TestBuildClient_ConvertsDefaultTotalGBToBytes(t *testing.T) {
	j := NewLdapSyncJob()
	c := j.buildClient("user@example.com", 10, 0, 0)
	if want := int64(10) * 1024 * 1024 * 1024; c.TotalGB != want {
		t.Errorf("TotalGB = %d, want %d", c.TotalGB, want)
	}
}

func TestLdapCreateClients_AttachesToAllConfiguredInbounds(t *testing.T) {
	initLdapJobDB(t)
	db := database.GetDB()

	tags := []string{"in-1080-tcp", "in-1081-tcp", "in-1082-tcp"}
	protocols := []model.Protocol{model.VLESS, model.Trojan, model.VLESS}
	inboundIds := make([]int, 0, len(tags))
	for i, tag := range tags {
		ib := &model.Inbound{
			UserId:         1,
			Tag:            tag,
			Enable:         true,
			Port:           42080 + i,
			Protocol:       protocols[i],
			Settings:       `{"clients": []}`,
			StreamSettings: `{"network":"tcp","security":"none"}`,
		}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", tag, err)
		}
		inboundIds = append(inboundIds, ib.Id)
	}

	j := NewLdapSyncJob()
	const email = "user@example.com"
	j.createClients([]model.Client{j.buildClient(email, 0, 0, 0)}, inboundIds, tags)

	rec := &model.ClientRecord{}
	if err := db.Where("email = ?", email).First(rec).Error; err != nil {
		t.Fatalf("client record for %s not created: %v", email, err)
	}
	if rec.SubID == "" {
		t.Error("created LDAP client must carry a subId")
	}

	clientSvc := &service.ClientService{}
	for i, id := range inboundIds {
		clients, err := clientSvc.ListForInbound(nil, id)
		if err != nil {
			t.Fatalf("ListForInbound(%s): %v", tags[i], err)
		}
		if len(clients) != 1 || clients[0].Email != email {
			t.Fatalf("inbound %s must carry exactly the LDAP client, got %d clients", tags[i], len(clients))
		}
		if clients[0].SubID != rec.SubID {
			t.Errorf("inbound %s client subId = %q, want the shared %q", tags[i], clients[0].SubID, rec.SubID)
		}
	}

	trojanClients, err := clientSvc.ListForInbound(nil, inboundIds[1])
	if err != nil {
		t.Fatalf("ListForInbound(trojan): %v", err)
	}
	if trojanClients[0].Password == "" {
		t.Error("trojan inbound client must get a generated password")
	}
	vlessClients, err := clientSvc.ListForInbound(nil, inboundIds[0])
	if err != nil {
		t.Fatalf("ListForInbound(vless): %v", err)
	}
	if vlessClients[0].ID == "" {
		t.Error("vless inbound client must get a generated uuid")
	}
}

// TestLdapCreateClients_FlagsRestartWhenEveryClientPartlyApplies pins that the
// restart survives created == 0, the case a partly-applied batch always hits.
func TestLdapCreateClients_FlagsRestartWhenEveryClientPartlyApplies(t *testing.T) {
	initLdapJobDB(t)
	db := database.GetDB()

	healthy := &model.Inbound{
		UserId: 1, Tag: "in-42180-tcp", Enable: true, Port: 42180,
		Protocol: model.VLESS, Settings: `{"clients": []}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	broken := &model.Inbound{
		UserId: 1, Tag: "in-42181-tcp", Enable: true, Port: 42181,
		Protocol: model.VLESS, Settings: `{"clients":`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	for _, ib := range []*model.Inbound{healthy, broken} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	j := NewLdapSyncJob()
	j.xrayService.IsNeedRestartAndSetFalse()
	j.createClients([]model.Client{j.buildClient("partial@example.com", 0, 0, 0)},
		[]int{healthy.Id, broken.Id}, []string{healthy.Tag, broken.Tag})

	clients, err := (&service.ClientService{}).ListForInbound(nil, healthy.Id)
	if err != nil {
		t.Fatalf("ListForInbound(%s): %v", healthy.Tag, err)
	}
	if len(clients) != 1 {
		t.Fatalf("healthy inbound holds %d clients, want the partly-applied 1", len(clients))
	}
	if !j.xrayService.IsNeedRestartAndSetFalse() {
		t.Fatal("a partly-applied LDAP batch left Xray unflagged for restart")
	}
}

// ldapDisabledClientsInbound seeds one local inbound holding the given clients
// switched off everywhere, each with its own client_traffics row.
func ldapDisabledClientsInbound(t *testing.T, rows []xray.ClientTraffic) *model.Inbound {
	t.Helper()
	initLdapJobDB(t)
	db := database.GetDB()
	clients := make([]model.Client, 0, len(rows))
	for i, row := range rows {
		clients = append(clients, model.Client{
			Email: row.Email, ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i),
			Enable: false, TotalGB: row.Total, ExpiryTime: row.ExpiryTime,
		})
	}
	settings, _ := json.Marshal(map[string]any{"clients": clients, "decryption": "none"})
	ib := &model.Inbound{
		UserId: 1, Enable: true, Port: 47300, Protocol: model.VLESS,
		Tag: "ldap-local-in", Settings: string(settings), StreamSettings: `{"network":"tcp"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	for i, c := range clients {
		rec := model.ClientRecord{Email: c.Email, UUID: c.ID, Enable: false, TotalGB: c.TotalGB, ExpiryTime: c.ExpiryTime}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("create client record: %v", err)
		}
		// gorm default:true drops an explicit false on Create (#6478).
		if err := db.Model(&rec).Update("enable", false).Error; err != nil {
			t.Fatalf("disable client record: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("link client: %v", err)
		}
		traffic := rows[i]
		traffic.InboundId = ib.Id
		if err := db.Create(&traffic).Error; err != nil {
			t.Fatalf("create client traffic: %v", err)
		}
	}
	return ib
}

// The directory flag stays true after an account expires or spends its quota, so
// enabling on the flag alone undid the traffic job's disable on every sync.
func TestLdapEnableLeavesDepletedClientsOff(t *testing.T) {
	ib := ldapDisabledClientsInbound(t, []xray.ClientTraffic{
		{Email: "expired@ldap", ExpiryTime: time.Now().Add(-time.Minute).UnixMilli()},
		{Email: "spent@ldap", Total: 100, Up: 60, Down: 40},
		{Email: "paused@ldap", Total: 100, Up: 1, ExpiryTime: time.Now().Add(time.Hour).UnixMilli()},
		{Email: "unlimited@ldap", Up: 1 << 40},
	})

	NewLdapSyncJob().batchSetEnable([]string{"expired@ldap", "spent@ldap", "paused@ldap", "unlimited@ldap"}, true)

	want := map[string]bool{"expired@ldap": false, "spent@ldap": false, "paused@ldap": true, "unlimited@ldap": true}
	for email, enabled := range inboundClientEnables(t, ib.Id) {
		if enabled != want[email] {
			t.Errorf("client %s enabled = %v after the LDAP enable, want %v", email, enabled, want[email])
		}
	}
}
