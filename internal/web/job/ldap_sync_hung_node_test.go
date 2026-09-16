package job

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// ldapHungNodeInbound seeds an online node whose client writes hang until the gate
// opens, plus one inbound on it holding the given enabled clients.
func ldapHungNodeInbound(t *testing.T, emails []string) (*resetGate, *model.Inbound) {
	t.Helper()
	initLdapJobDB(t)
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })
	gate := &resetGate{release: make(chan struct{})}
	const tag = "ldap-node-in"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "inbounds/list") {
			_, _ = w.Write([]byte(`{"success":true,"obj":[{"id":1,"tag":"` + tag + `"}]}`))
			return
		}
		if r.Method == http.MethodPost {
			gate.entered.Add(1)
			select {
			case <-r.Context().Done():
			case <-gate.release:
			}
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(gate.open)
	host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	portNum, _ := strconv.Atoi(port)
	db := database.GetDB()
	node := &model.Node{
		Name: "ldap-node", Scheme: "http", Address: host, Port: portNum, BasePath: "/", ApiToken: "tok",
		Enable: true, Status: "online", AllowPrivateAddress: true, TlsVerifyMode: "verify",
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	clients := make([]model.Client, 0, len(emails))
	for i, email := range emails {
		clients = append(clients, model.Client{Email: email, ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i), Enable: true})
	}
	settings, _ := json.Marshal(map[string]any{"clients": clients, "decryption": "none"})
	ib := &model.Inbound{
		UserId: 1, Enable: true, Port: 47200, Protocol: model.VLESS, NodeID: &node.Id,
		Tag: tag, Settings: string(settings), StreamSettings: `{"network":"tcp"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	for _, c := range clients {
		rec := model.ClientRecord{Email: c.Email, UUID: c.ID, Enable: true}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("create client record: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("link client: %v", err)
		}
		if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: c.Email, Enable: true}).Error; err != nil {
			t.Fatalf("create client traffic: %v", err)
		}
	}
	return gate, ib
}

func inboundClientEnables(t *testing.T, inboundID int) map[string]bool {
	t.Helper()
	var ib model.Inbound
	if err := database.GetDB().First(&ib, inboundID).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var settings struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	out := make(map[string]bool, len(settings.Clients))
	for _, c := range settings.Clients {
		out[c.Email] = c.Enable
	}
	return out
}

// Each LDAP user was disabled on its own, and every per-user push held the inbound
// lock for the push timeout, so users sharing a hung node inbound queued behind it.
func TestLdapBatchSetEnableDoesNotQueueUsersOnHungNode(t *testing.T) {
	emails := []string{"u1@ldap", "u2@ldap", "u3@ldap", "u4@ldap", "u5@ldap"}
	gate, ib := ldapHungNodeInbound(t, emails)

	done := make(chan struct{})
	go func() {
		defer close(done)
		NewLdapSyncJob().batchSetEnable(emails, false)
	}()
	t.Cleanup(func() { gate.open(); <-done })
	select {
	case <-done:
	case <-time.After(9 * time.Second):
		t.Fatalf("disabling %d LDAP users on one hung node inbound took over 9s (%d pushes started)", len(emails), gate.entered.Load())
	}
	for email, enabled := range inboundClientEnables(t, ib.Id) {
		if enabled {
			t.Errorf("client %s still enabled after the LDAP disable", email)
		}
	}
}

// Clients missing from LDAP were detached one at a time, each waiting out the push
// timeout on a hung node inbound, so a directory cleanup could run for hours.
func TestLdapDeleteDoesNotQueueClientsOnHungNode(t *testing.T) {
	emails := []string{"gone1@ldap", "gone2@ldap", "gone3@ldap", "gone4@ldap", "gone5@ldap"}
	gate, ib := ldapHungNodeInbound(t, emails)

	done := make(chan struct{})
	go func() {
		defer close(done)
		NewLdapSyncJob().deleteClientsNotInLDAP(ib.Tag, map[string]struct{}{})
	}()
	t.Cleanup(func() { gate.open(); <-done })
	select {
	case <-done:
	case <-time.After(9 * time.Second):
		t.Fatalf("detaching %d clients from one hung node inbound took over 9s (%d pushes started)", len(emails), gate.entered.Load())
	}
	if left := inboundClientEnables(t, ib.Id); len(left) != 0 {
		t.Errorf("clients still on the inbound after the LDAP cleanup: %v", left)
	}
}
