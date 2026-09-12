package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type mockXrayRestart struct {
	restarted bool
	err       error
}

func (m *mockXrayRestart) RestartXray(force bool) error {
	m.restarted = true
	return m.err
}

func TestGatewayClient_EndToEndCommands(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotEnable(true)
	_ = settingService.SetDiscordBotToken("test-gw-token")
	_ = settingService.SetDiscordChannelId("ch-12345")

	var sentMessages []MessagePayload
	var mu sync.Mutex

	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p MessagePayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		mu.Lock()
		sentMessages = append(sentMessages, p)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "msg-sent"}`))
	}))
	defer restServer.Close()

	discordSvc := NewDiscordService(settingService)
	discordSvc.SetBaseURL(restServer.URL)
	discordSvc.SetHTTPClient(restServer.Client())

	upgrader := websocket.Upgrader{}
	wsConnected := make(chan struct{})

	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// 1. Send Op 10 Hello
		hello := GatewayPayload{
			Op: opHello,
			D:  []byte(`{"heartbeat_interval": 500}`),
		}
		_ = conn.WriteJSON(hello)

		// 2. Read Op 2 Identify
		var ident GatewayPayload
		_ = conn.ReadJSON(&ident)

		close(wsConnected)

		// 3. Send !help message
		helpMsg := MessageCreateData{
			ID:        "m1",
			ChannelID: "ch-12345",
			Content:   "!help",
			Author: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Bot      bool   `json:"bot"`
			}{ID: "u1", Username: "Alice", Bot: false},
		}
		helpBytes, _ := json.Marshal(helpMsg)
		_ = conn.WriteJSON(GatewayPayload{
			Op: opDispatch,
			T:  "MESSAGE_CREATE",
			D:  helpBytes,
		})

		time.Sleep(50 * time.Millisecond)

		// 4. Send !status message
		statusMsg := MessageCreateData{
			ID:        "m2",
			ChannelID: "ch-12345",
			Content:   "!status",
			Author: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Bot      bool   `json:"bot"`
			}{ID: "u1", Username: "Alice", Bot: false},
		}
		statusBytes, _ := json.Marshal(statusMsg)
		_ = conn.WriteJSON(GatewayPayload{
			Op: opDispatch,
			T:  "MESSAGE_CREATE",
			D:  statusBytes,
		})

		time.Sleep(50 * time.Millisecond)

		// 5. Send message from a bot (must be ignored)
		botMsg := MessageCreateData{
			ID:        "m3",
			ChannelID: "ch-12345",
			Content:   "!status",
			Author: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Bot      bool   `json:"bot"`
			}{ID: "u2", Username: "OtherBot", Bot: true},
		}
		botBytes, _ := json.Marshal(botMsg)
		_ = conn.WriteJSON(GatewayPayload{
			Op: opDispatch,
			T:  "MESSAGE_CREATE",
			D:  botBytes,
		})

		time.Sleep(50 * time.Millisecond)

		// 6. Send !usage for existing client
		usageMsg := MessageCreateData{
			ID:        "m4",
			ChannelID: "ch-12345",
			Content:   "!usage client@test.com",
			Author: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Bot      bool   `json:"bot"`
			}{ID: "u1", Username: "Alice", Bot: false},
		}
		usageBytes, _ := json.Marshal(usageMsg)
		_ = conn.WriteJSON(GatewayPayload{
			Op: opDispatch,
			T:  "MESSAGE_CREATE",
			D:  usageBytes,
		})

		time.Sleep(50 * time.Millisecond)

		// 7. Send !restart command
		restartMsg := MessageCreateData{
			ID:        "m5",
			ChannelID: "ch-12345",
			Content:   "!restart",
			Author: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Bot      bool   `json:"bot"`
			}{ID: "u1", Username: "Alice", Bot: false},
		}
		restartBytes, _ := json.Marshal(restartMsg)
		_ = conn.WriteJSON(GatewayPayload{
			Op: opDispatch,
			T:  "MESSAGE_CREATE",
			D:  restartBytes,
		})

		// Keep connection alive until closed
		for {
			var p GatewayPayload
			if err := conn.ReadJSON(&p); err != nil {
				break
			}
		}
	}))
	defer wsServer.Close()

	mockServer := &mockServerProvider{
		status: &service.Status{
			Uptime:   10000,
			Loads:    []float64{0.1, 0.2, 0.3},
			TcpCount: 5,
			UdpCount: 2,
		},
	}
	mockInbound := &mockInboundProvider{
		inbounds: []*model.Inbound{
			{
				Id:       1,
				Remark:   "VLESS-Test",
				Port:     8443,
				Protocol: "vless",
				Enable:   true,
				ClientStats: []xray.ClientTraffic{
					{
						Email:  "client@test.com",
						Enable: true,
						Up:     1024,
						Down:   2048,
						Total:  10485760,
					},
				},
			},
		},
	}
	mockXray := &mockXrayRestart{}

	wsURL := "ws" + strings.TrimPrefix(wsServer.URL, "http")

	gw := NewGatewayClient(discordSvc, settingService, mockServer, mockInbound, mockXray)
	gw.SetGatewayURL(wsURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := gw.Start(ctx); err != nil {
		t.Fatalf("gw.Start failed: %v", err)
	}

	select {
	case <-wsConnected:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for WS connection")
	}

	// Wait for dispatches to be processed
	time.Sleep(300 * time.Millisecond)

	gw.Stop()

	if gw.IsRunning() {
		t.Error("expected gateway not to be running after Stop")
	}

	mu.Lock()
	msgs := make([]MessagePayload, len(sentMessages))
	copy(msgs, sentMessages)
	mu.Unlock()

	// We expect:
	// 1. !help response embed
	// 2. !status response embed
	// (bot message ignored)
	// 3. !usage response embed
	// 4. !restart "Restarting..." and "Restarted successfully"
	if len(msgs) < 4 {
		t.Fatalf("expected at least 4 message responses, got %d: %+v", len(msgs), msgs)
	}

	foundHelp := false
	foundStatus := false
	foundUsage := false
	for _, m := range msgs {
		for _, e := range m.Embeds {
			if strings.Contains(e.Title, "Discord Bot Commands") {
				foundHelp = true
			}
			if strings.Contains(e.Title, "Server Status") {
				foundStatus = true
			}
			if strings.Contains(e.Title, "Client Usage: client@test.com") {
				foundUsage = true
			}
		}
	}

	if !foundHelp {
		t.Error("expected help embed to be sent")
	}
	if !foundStatus {
		t.Error("expected status embed to be sent")
	}
	if !foundUsage {
		t.Error("expected usage embed to be sent")
	}
	if !mockXray.restarted {
		t.Error("expected Xray core to be restarted")
	}
}
