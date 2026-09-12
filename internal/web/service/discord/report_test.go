package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type mockServerProvider struct {
	status   *service.Status
	dbData   []byte
	dbErr    error
	filename string
}

func (m *mockServerProvider) GetStatus(lastStatus *service.Status) *service.Status {
	return m.status
}

func (m *mockServerProvider) GetDb() ([]byte, error) {
	return m.dbData, m.dbErr
}

func (m *mockServerProvider) BackupFilename(requestHost string) string {
	if m.filename != "" {
		return m.filename
	}
	return "x-ui_test.db"
}

type mockInboundProvider struct {
	inbounds []*model.Inbound
	err      error
}

func (m *mockInboundProvider) GetAllInbounds() ([]*model.Inbound, error) {
	return m.inbounds, m.err
}

func TestBuildReport_NoBackup(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("test-bot-token")
	_ = settingService.SetDiscordChannelId("12345")
	_ = settingService.SetDiscordBotBackup(false)
	_ = settingService.SetDiscordRunTime("@daily")

	mockStatus := &service.Status{
		Uptime:   172800,
		Loads:    []float64{0.5, 0.4, 0.3},
		TcpCount: 15,
		UdpCount: 5,
	}
	mockStatus.Xray.State = service.Running
	mockStatus.Xray.Version = "25.1.0"

	mockServer := &mockServerProvider{
		status: mockStatus,
		dbData: []byte("sqlite-backup-bytes"),
	}

	mockInbound := &mockInboundProvider{
		inbounds: []*model.Inbound{
			{
				Id:     1,
				Remark: "VLESS-TCP",
				Enable: true,
				Port:   443,
				ClientStats: []xray.ClientTraffic{
					{Email: "user1@example.com", Enable: true, Up: 100, Down: 200},
					{Email: "user2@example.com", Enable: false},
				},
			},
		},
	}

	svc := NewDiscordService(settingService)
	payload, files, err := svc.BuildReport(context.Background(), mockServer, mockInbound)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}

	if len(payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(payload.Embeds))
	}
	embed := payload.Embeds[0]
	if embed.Color != ColorBlue {
		t.Errorf("expected ColorBlue, got %X", embed.Color)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files when backup is disabled, got %d", len(files))
	}

	foundHost, foundUptime, foundXray := false, false, false
	for _, field := range embed.Fields {
		if field.Name == "Host" {
			foundHost = true
		}
		if field.Name == "Uptime" && field.Value == "2d 0h" {
			foundUptime = true
		}
		if field.Name == "Xray Core" && field.Value == "25.1.0 (running)" {
			foundXray = true
		}
	}
	if !foundHost || !foundUptime || !foundXray {
		t.Errorf("expected fields not found in embed: %+v", embed.Fields)
	}
}

func TestBuildReport_WithBackup(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("test-bot-token")
	_ = settingService.SetDiscordChannelId("12345")
	_ = settingService.SetDiscordBotBackup(true)

	mockStatus := &service.Status{
		Uptime: 3600,
	}
	mockStatus.Xray.State = service.Running
	mockStatus.Xray.Version = "25.1.0"

	mockServer := &mockServerProvider{
		status:   mockStatus,
		dbData:   []byte("test-db-content"),
		filename: "x-ui_backup.db",
	}

	svc := NewDiscordService(settingService)
	_, files, err := svc.BuildReport(context.Background(), mockServer, nil)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least 1 backup file, got 0")
	}
	if files[0].Filename != "x-ui_backup.db" {
		t.Errorf("expected filename 'x-ui_backup.db', got %q", files[0].Filename)
	}
	if string(files[0].Data) != "test-db-content" {
		t.Errorf("expected db content 'test-db-content', got %q", string(files[0].Data))
	}
}

func TestSendReport_Integration(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("test-token")
	_ = settingService.SetDiscordChannelId("998877")
	_ = settingService.SetDiscordBotBackup(true)

	var receivedRequest bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "msg-123"}`))
	}))
	defer server.Close()

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	mockStatus := &service.Status{
		Uptime: 86400,
	}
	mockStatus.Xray.State = service.Running
	mockStatus.Xray.Version = "25.1.0"

	mockServer := &mockServerProvider{
		status: mockStatus,
		dbData: []byte("sqlite-data"),
	}

	err := svc.SendReport(context.Background(), mockServer, nil)
	if err != nil {
		t.Fatalf("SendReport failed: %v", err)
	}
	if !receivedRequest {
		t.Error("expected server to receive report request")
	}
}
