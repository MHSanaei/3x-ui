package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type reportStatus struct{ status *service.Status }

func (provider reportStatus) GetStatus(*service.Status) *service.Status { return provider.status }

type reportInbounds struct {
	inbounds []*model.Inbound
	err      error
}

func (provider reportInbounds) GetAllInbounds() ([]*model.Inbound, error) {
	return provider.inbounds, provider.err
}

func TestReportSendsOnlySummaryAndCountsSharedClientsOnce(t *testing.T) {
	settings := webhookSettings(t)
	now := time.Now().UnixMilli()
	status := &service.Status{Cpu: 12.5, Uptime: 90000}
	status.Mem.Current, status.Mem.Total = 1<<30, 4<<30
	status.Disk.Current, status.Disk.Total = 10<<30, 50<<30
	status.Xray.Version = "test-core"
	items := []*model.Inbound{
		{Id: 1, Enable: true, ExpiryTime: now - 1000, Settings: `{"privateKey":"never-send"}`, ClientStats: []xray.ClientTraffic{
			{Email: "shared", Enable: true, Total: 100, Up: 100},
			{Email: "expired", Enable: true, ExpiryTime: now - 1000},
		}},
		{Id: 2, Enable: true, Total: 100, Down: 100, ClientStats: []xray.ClientTraffic{{Email: "shared", Enable: true, Total: 100, Up: 100}}},
	}
	var payload Envelope
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
	})
	if err := svc.SendReport(context.Background(), reportStatus{status}, reportInbounds{inbounds: items}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"CPU: 12.50%", "1.00GB / 4.00GB", "10.00GB / 50.00GB", "Total: 2", "Expired: 1", "Depleted: 1"} {
		if !strings.Contains(payload.Text, expected) {
			t.Errorf("report missing %q: %s", expected, payload.Text)
		}
	}
	if payload.Type != "report" || payload.Event != "report" || strings.Contains(payload.Text, "never-send") || strings.Contains(payload.Text, "shared") {
		t.Fatalf("report leaked configuration or client identity: %+v", payload)
	}
}

func TestReportDoesNotPresentUnavailableMetricsAsMeasuredZero(t *testing.T) {
	settings := webhookSettings(t)
	var payload Envelope
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { _ = json.NewDecoder(r.Body).Decode(&payload) })
	status := &service.Status{UnavailableMetrics: []string{"cpu", "uptime"}}
	if err := svc.SendReport(context.Background(), reportStatus{status}, reportInbounds{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.Text, "CPU: —") || !strings.Contains(payload.Text, "Uptime: —") || strings.Contains(payload.Text, "CPU: 0.00%") {
		t.Fatalf("unavailable metric became a real zero: %s", payload.Text)
	}
}

func TestReportReadFailureDoesNotSendAnAllZeroSuccessReport(t *testing.T) {
	settings := webhookSettings(t)
	calls := 0
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	err := svc.SendReport(context.Background(), reportStatus{&service.Status{}}, reportInbounds{err: errors.New("private database path")})
	if !errors.Is(err, ErrReport) || calls != 0 || strings.Contains(err.Error(), "private database path") {
		t.Fatalf("report failure was sent or disclosed: calls=%d err=%v", calls, err)
	}
}

func TestReportIncludesSubscriptionUsedAndRemainingTraffic(t *testing.T) {
	for _, tc := range []struct {
		name            string
		items           []*model.Inbound
		used, remaining string
	}{
		{"shared subscription", []*model.Inbound{
			{Up: 1 << 40, ClientStats: []xray.ClientTraffic{{Email: "a", Up: 1024, Down: 2048, Total: 8192}}},
			{Up: 1 << 40, ClientStats: []xray.ClientTraffic{{Email: "a", Up: 1024, Down: 2048, Total: 8192}, {Email: "b", Up: 8192, Total: 4096}}},
		}, "11.00KB", "5.00KB"},
		{"not configured", []*model.Inbound{{Total: 8192, ClientStats: []xray.ClientTraffic{{Email: "a", Up: 1024, Down: 2048}}}}, "3.00KB", "Quota not configured"},
		{"partly configured", []*model.Inbound{{ClientStats: []xray.ClientTraffic{{Email: "a", Up: 1024, Total: 4096}, {Email: "b", Down: 2048}}}}, "3.00KB", "Quota not configured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := webhookSettings(t)
			var payload Envelope
			svc := NewService(settings)
			svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { _ = json.NewDecoder(r.Body).Decode(&payload) })
			if err := svc.SendReport(context.Background(), reportStatus{&service.Status{}}, reportInbounds{inbounds: tc.items}); err != nil {
				t.Fatal(err)
			}
			for _, expected := range []string{"3x-ui used traffic: " + tc.used, "3x-ui remaining traffic: " + tc.remaining} {
				if !strings.Contains(payload.Text, expected) {
					t.Errorf("missing %q in report: %s", expected, payload.Text)
				}
			}
		})
	}
}

func TestReportUsesNormalizedLimitsWhenNodeTrafficOmitsThem(t *testing.T) {
	settings := webhookSettings(t)
	db := database.GetDB()
	item := model.Inbound{Enable: true, Settings: "{}"}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	client := model.ClientRecord{Email: "private-client", UUID: "private-uuid", TotalGB: 8192, ExpiryTime: time.Now().UnixMilli() - 1000, Enable: true}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	runtime := xray.ClientTraffic{InboundId: item.Id, Email: client.Email, Enable: true, Up: 1024, Down: 2048}
	if err := db.Create(&runtime).Error; err != nil {
		t.Fatal(err)
	}
	var payload Envelope
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { _ = json.NewDecoder(r.Body).Decode(&payload) })
	if err := svc.SendReport(context.Background(), reportStatus{&service.Status{}}, &service.InboundService{}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"3x-ui used traffic: 3.00KB", "3x-ui remaining traffic: 5.00KB", "Clients: Total: 1 | Disabled: 0 | Expired: 1 | Depleted: 0"} {
		if !strings.Contains(payload.Text, expected) {
			t.Errorf("report missing %q: %s", expected, payload.Text)
		}
	}
	if strings.Contains(payload.Text, client.Email) || strings.Contains(payload.Text, client.UUID) {
		t.Fatal("report disclosed client identity")
	}
	var saved xray.ClientTraffic
	if err := db.First(&saved, runtime.Id).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Total != 0 || saved.ExpiryTime != 0 {
		t.Fatal("report changed the runtime counters")
	}
}

func TestReportNormalizedLimitReadFailureDoesNotSend(t *testing.T) {
	settings := webhookSettings(t)
	if err := database.GetDB().Migrator().DropTable(&model.ClientRecord{}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	items := []*model.Inbound{{ClientStats: []xray.ClientTraffic{{Email: "node-client", Up: 1024}}}}
	err := svc.SendReport(context.Background(), reportStatus{&service.Status{}}, reportInbounds{inbounds: items})
	if !errors.Is(err, ErrReport) || calls != 0 {
		t.Fatalf("unavailable limits produced a successful report: calls=%d err=%v", calls, err)
	}
}

func TestReportPreservesNonzeroRuntimeLimits(t *testing.T) {
	settings := webhookSettings(t)
	client := model.ClientRecord{Email: "node-client", TotalGB: 8192, ExpiryTime: time.Now().UnixMilli() - 1000}
	if err := database.GetDB().Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	items := []*model.Inbound{{ClientStats: []xray.ClientTraffic{{Email: client.Email, Enable: true, Up: 1024, Down: 2048, Total: 4096, ExpiryTime: time.Now().Add(time.Hour).UnixMilli()}}}}
	var payload Envelope
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { _ = json.NewDecoder(r.Body).Decode(&payload) })
	if err := svc.SendReport(context.Background(), reportStatus{&service.Status{}}, reportInbounds{inbounds: items}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.Text, "3x-ui remaining traffic: 1.00KB") || !strings.Contains(payload.Text, "Clients: Total: 1 | Disabled: 0 | Expired: 0 | Depleted: 0") {
		t.Fatalf("normalized limits replaced available runtime limits: %s", payload.Text)
	}
}

func TestReportCaseInsensitiveClientIdentity(t *testing.T) {
	for _, tc := range []struct {
		name       string
		total      int64
		hasExpiry  bool
		remaining  string
		expired    int
		email      string
		statsEmail string
	}{
		{name: "both limits omitted", remaining: "5.00KB", expired: 1},
		{name: "runtime quota preserved", total: 4096, remaining: "1.00KB", expired: 1},
		{name: "runtime expiry preserved", hasExpiry: true, remaining: "5.00KB"},
		{name: "both runtime limits preserved", total: 4096, hasExpiry: true, remaining: "1.00KB"},
		{name: "unicode client identity", email: "ПРИВАТНЫЙ-КЛИЕНТ", remaining: "5.00KB", expired: 1},
		{name: "unicode title identity", email: "Приватный-клиент", remaining: "5.00KB", expired: 1},
		{name: "unicode original identity", email: "пРиВаТнЫй-КлИеНт", statsEmail: "пРиВаТнЫй-КлИеНт", remaining: "5.00KB", expired: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := webhookSettings(t)
			db := database.GetDB()
			email := tc.email
			if email == "" {
				email = "Private-Client"
			}
			client := model.ClientRecord{Email: email, TotalGB: 8192, ExpiryTime: time.Now().Add(-time.Hour).UnixMilli()}
			if err := db.Create(&client).Error; err != nil {
				t.Fatal(err)
			}
			var originals []xray.ClientTraffic
			statsEmails := []string{strings.ToLower(email), strings.ToUpper(email)}
			if tc.statsEmail != "" {
				statsEmails[0] = tc.statsEmail
			}
			for _, email := range statsEmails {
				inbound := model.Inbound{Tag: fmt.Sprintf("casefold-%d", len(originals)), Enable: true, Settings: "{}"}
				if err := db.Create(&inbound).Error; err != nil {
					t.Fatal(err)
				}
				runtime := xray.ClientTraffic{InboundId: inbound.Id, Email: email, Enable: true, Up: 1024, Down: 2048, Total: tc.total}
				if tc.hasExpiry {
					runtime.ExpiryTime = time.Now().Add(time.Hour).UnixMilli()
				}
				if err := db.Create(&runtime).Error; err != nil {
					t.Fatal(err)
				}
				originals = append(originals, runtime)
			}
			var payload Envelope
			svc := NewService(settings)
			svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { _ = json.NewDecoder(r.Body).Decode(&payload) })
			if err := svc.SendReport(context.Background(), reportStatus{&service.Status{}}, &service.InboundService{}); err != nil {
				t.Fatal(err)
			}
			for _, expected := range []string{
				"3x-ui used traffic: 3.00KB",
				"3x-ui remaining traffic: " + tc.remaining,
				fmt.Sprintf("Clients: Total: 1 | Disabled: 0 | Expired: %d | Depleted: 0", tc.expired),
			} {
				if !strings.Contains(payload.Text, expected) {
					t.Errorf("report missing %q: %s", expected, payload.Text)
				}
			}
			for _, original := range originals {
				var saved xray.ClientTraffic
				if err := db.First(&saved, original.Id).Error; err != nil {
					t.Fatal(err)
				}
				if saved != original {
					t.Errorf("report changed runtime row: got %+v, want %+v", saved, original)
				}
			}
		})
	}
}

func TestReportClientLimitsLoadsCaseInsensitiveBatch(t *testing.T) {
	webhookSettings(t)
	clients := make([]model.ClientRecord, 501)
	stats := make([]xray.ClientTraffic, 0, 2*len(clients))
	for i := range clients {
		email := fmt.Sprintf("Client-%03d", i)
		clients[i] = model.ClientRecord{Email: email, TotalGB: int64(i + 1), ExpiryTime: int64(i + 2)}
		stats = append(stats, xray.ClientTraffic{Email: strings.ToUpper(email)}, xray.ClientTraffic{Email: strings.ToLower(email)})
	}
	if err := database.GetDB().CreateInBatches(&clients, 100).Error; err != nil {
		t.Fatal(err)
	}
	const callback = "test:report_limit_parameter_budget"
	db := database.GetDB()
	queries := 0
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "clients" {
			queries++
			if got := len(tx.Statement.Vars); got > 500 {
				t.Errorf("client limit query used %d parameters, want at most 500", got)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	limits, err := reportClientLimits(context.Background(), []*model.Inbound{{ClientStats: stats}})
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != len(clients) {
		t.Fatalf("loaded %d client limits, want %d", len(limits), len(clients))
	}
	if queries == 0 {
		t.Fatal("test did not observe a client limit query")
	}
	for _, client := range clients {
		if got := limits[strings.ToLower(client.Email)]; got.Email != client.Email || got.TotalGB != client.TotalGB || got.ExpiryTime != client.ExpiryTime {
			t.Errorf("incorrect client limits for %q: %+v", client.Email, got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reportClientLimits(ctx, []*model.Inbound{{ClientStats: stats}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("limit lookup ignored cancelled context: %v", err)
	}
}
