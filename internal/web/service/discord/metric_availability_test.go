package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestReviewDiscordUnavailableUptime(t *testing.T) {
	for _, unavailable := range []bool{true, false} {
		t.Run(map[bool]string{true: "unavailable", false: "valid zero"}[unavailable], func(t *testing.T) {
			settings := setupTestDB(t)
			_ = settings.SetDiscordLang("en-US")
			_ = settings.SetDiscordBotBackup(false)
			status := &service.Status{Loads: []float64{0, 0, 0}}
			if unavailable {
				status.UnavailableMetrics = []string{"uptime"}
			}
			server := &mockServerProvider{status: status}
			svc := NewDiscordService(settings)
			report, _, err := svc.BuildReport(context.Background(), server, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := "0d 0h"
			if unavailable {
				want = "—"
			}
			check := func(fields []EmbedField) {
				t.Helper()
				for _, f := range fields {
					if f.Name == "Uptime" {
						if f.Value != want {
							t.Fatalf("uptime=%q want %q", f.Value, want)
						}
						return
					}
				}
				t.Fatal("uptime field missing")
			}
			check(report.Embeds[0].Fields)
			_ = settings.SetDiscordBotToken("synthetic-test")
			_ = settings.SetDiscordChannelId("synthetic-channel")
			sent := make(chan MessagePayload, 1)
			rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload MessagePayload
				_ = json.NewDecoder(r.Body).Decode(&payload)
				sent <- payload
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"synthetic-message"}`))
			}))
			defer rest.Close()
			svc.SetBaseURL(rest.URL)
			svc.SetHTTPClient(rest.Client())
			g := NewGatewayClient(svc, settings, server, nil, nil)
			g.sendStatus(context.Background())
			select {
			case msg := <-sent:
				check(msg.Embeds[0].Fields)
			default:
				t.Fatal("no local status payload")
			}
		})
	}
}
