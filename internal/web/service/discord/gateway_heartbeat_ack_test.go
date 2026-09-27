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
)

func TestGatewayDropsConnectionWhenHeartbeatsGoUnanswered(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotEnable(true)
	_ = settingService.SetDiscordBotToken("test-gw-token")

	upgrader := websocket.Upgrader{}
	clientClosed := make(chan struct{})
	var once sync.Once
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Hello with a short interval and not one op 11 after it: the socket stays
		// open and reads fine, which is what a zombied connection looks like.
		hello := GatewayPayload{Op: opHello}
		hello.D, _ = json.Marshal(HelloData{HeartbeatInterval: 50})
		if err := conn.WriteJSON(hello); err != nil {
			return
		}
		for {
			var payload GatewayPayload
			if err := conn.ReadJSON(&payload); err != nil {
				once.Do(func() { close(clientClosed) })
				return
			}
		}
	}))
	defer wsServer.Close()

	discordSvc := NewDiscordService(settingService)
	gw := NewGatewayClient(discordSvc, settingService, &mockServerProvider{}, &mockInboundProvider{}, &mockXrayRestart{})
	gw.SetGatewayURL("ws" + strings.TrimPrefix(wsServer.URL, "http"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := gw.Start(ctx); err != nil {
		t.Fatalf("gw.Start failed: %v", err)
	}
	defer gw.Stop()

	select {
	case <-clientClosed:
	case <-time.After(3 * time.Second):
		t.Fatal("gateway held on to a connection whose heartbeats were never acknowledged")
	}
}
