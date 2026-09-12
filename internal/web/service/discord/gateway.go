package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	defaultGatewayURL = "wss://gateway.discord.gg/?v=10&encoding=json"

	opDispatch     = 0
	opHeartbeat    = 1
	opIdentify     = 2
	opHello        = 10
	opHeartbeatACK = 11

	// GUILDS (1<<0) | GUILD_MESSAGES (1<<9) | DIRECT_MESSAGES (1<<12) | MESSAGE_CONTENT (1<<15)
	discordIntents = 37377
)

// GatewayPayload represents a Discord Gateway WebSocket frame.
type GatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  *int64          `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

// HelloData represents the payload received in Opcode 10 Hello.
type HelloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// IdentifyData represents the payload sent in Opcode 2 Identify.
type IdentifyData struct {
	Token      string             `json:"token"`
	Intents    int                `json:"intents"`
	Properties IdentifyProperties `json:"properties"`
}

// IdentifyProperties metadata for Discord identification.
type IdentifyProperties struct {
	OS      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

// MessageCreateData represents incoming message data from Discord.
type MessageCreateData struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
}

// XrayRestartProvider abstracts restarting the core.
type XrayRestartProvider interface {
	RestartXray(force bool) error
}

// GatewayClient manages the Discord Gateway WebSocket connection for interactive commands.
type GatewayClient struct {
	discordService *DiscordService
	settingService service.SettingService
	serverService  ServerProvider
	inboundService InboundProvider
	xrayService    XrayRestartProvider
	gatewayURL     string

	mu      sync.Mutex
	conn    *websocket.Conn
	cancel  context.CancelFunc
	running bool
	lastSeq *int64
}

// NewGatewayClient creates a new Discord Gateway client instance.
func NewGatewayClient(
	discordService *DiscordService,
	settingService service.SettingService,
	server ServerProvider,
	inbound InboundProvider,
	xray XrayRestartProvider,
) *GatewayClient {
	return &GatewayClient{
		discordService: discordService,
		settingService: settingService,
		serverService:  server,
		inboundService: inbound,
		xrayService:    xray,
		gatewayURL:     defaultGatewayURL,
	}
}

// SetGatewayURL overrides the gateway URL for testing.
func (g *GatewayClient) SetGatewayURL(url string) {
	g.gatewayURL = url
}

// IsRunning reports whether the Gateway client is active.
func (g *GatewayClient) IsRunning() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.running
}

// Start begins the Gateway connection and listening loop.
func (g *GatewayClient) Start(parentCtx context.Context) error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(parentCtx)
	g.cancel = cancel
	g.running = true
	g.mu.Unlock()

	go func() {
		defer func() {
			g.mu.Lock()
			g.running = false
			g.mu.Unlock()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			enabled, err := g.settingService.GetDiscordBotEnable()
			if err != nil || !enabled {
				return
			}

			err = g.connectAndListen(ctx)
			if err != nil && ctx.Err() == nil {
				logger.Warning("Discord Gateway disconnected: ", err, "; reconnecting in 5s...")
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}
	}()

	return nil
}

// Stop terminates the Gateway connection cleanly.
func (g *GatewayClient) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.running {
		return
	}
	if g.cancel != nil {
		g.cancel()
	}
	if g.conn != nil {
		_ = g.conn.Close()
	}
	g.running = false
}

func (g *GatewayClient) connectAndListen(ctx context.Context) error {
	token, err := g.settingService.GetDiscordBotToken()
	if err != nil || strings.TrimSpace(token) == "" {
		return errors.New("discord bot token not configured")
	}
	cleanToken := strings.TrimSpace(token)
	cleanToken = strings.TrimPrefix(cleanToken, "Bot ")
	cleanToken = strings.TrimSpace(cleanToken)

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.DialContext(ctx, g.gatewayURL, nil)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return fmt.Errorf("dial discord gateway: %w", err)
	}

	g.mu.Lock()
	g.conn = conn
	g.mu.Unlock()

	defer func() {
		_ = conn.Close()
		g.mu.Lock()
		if g.conn == conn {
			g.conn = nil
		}
		g.mu.Unlock()
	}()

	// 1. Read Hello opcode 10
	var helloPayload GatewayPayload
	if err := conn.ReadJSON(&helloPayload); err != nil {
		return fmt.Errorf("read hello payload: %w", err)
	}
	if helloPayload.Op != opHello {
		return fmt.Errorf("expected opcode 10, got %d", helloPayload.Op)
	}

	var helloData HelloData
	if err := json.Unmarshal(helloPayload.D, &helloData); err != nil {
		return fmt.Errorf("unmarshal hello data: %w", err)
	}

	// 2. Send Identify opcode 2
	identifyPayload := GatewayPayload{
		Op: opIdentify,
	}
	identData := IdentifyData{
		Token:   "Bot " + cleanToken,
		Intents: discordIntents,
		Properties: IdentifyProperties{
			OS:      "linux",
			Browser: "3x-ui",
			Device:  "3x-ui",
		},
	}
	dataBytes, _ := json.Marshal(identData)
	identifyPayload.D = dataBytes

	if err := conn.WriteJSON(identifyPayload); err != nil {
		return fmt.Errorf("send identify payload: %w", err)
	}

	// 3. Heartbeat loop
	hbStop := make(chan struct{})
	defer close(hbStop)

	go func() {
		interval := time.Duration(helloData.HeartbeatInterval) * time.Millisecond
		if interval <= 0 {
			interval = 40 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-hbStop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.mu.Lock()
				seq := g.lastSeq
				c := g.conn
				g.mu.Unlock()
				if c == nil {
					return
				}
				hb := GatewayPayload{Op: opHeartbeat}
				if seq != nil {
					seqBytes, _ := json.Marshal(*seq)
					hb.D = seqBytes
				}
				if err := c.WriteJSON(hb); err != nil {
					logger.Warning("Discord heartbeat write failed: ", err)
					return
				}
			}
		}
	}()

	// 4. Message dispatch loop
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var payload GatewayPayload
		if err := conn.ReadJSON(&payload); err != nil {
			return err
		}

		if payload.S != nil {
			g.mu.Lock()
			g.lastSeq = payload.S
			g.mu.Unlock()
		}

		switch payload.Op {
		case opHeartbeatACK:
			// Heartbeat acknowledged
		case opHeartbeat:
			// Discord requested immediate heartbeat
			g.mu.Lock()
			seq := g.lastSeq
			g.mu.Unlock()
			hb := GatewayPayload{Op: opHeartbeat}
			if seq != nil {
				seqBytes, _ := json.Marshal(*seq)
				hb.D = seqBytes
			}
			_ = conn.WriteJSON(hb)
		case opDispatch:
			if payload.T == "MESSAGE_CREATE" {
				var msg MessageCreateData
				if err := json.Unmarshal(payload.D, &msg); err == nil {
					go func(m MessageCreateData) {
						defer func() {
							if r := recover(); r != nil {
								logger.Error("Recovered panic in Discord message handler: ", r)
							}
						}()
						g.handleMessage(ctx, m)
					}(msg)
				}
			}
		}
	}
}

func (g *GatewayClient) handleMessage(ctx context.Context, msg MessageCreateData) {
	if msg.Author.Bot {
		return
	}
	channelID, err := g.settingService.GetDiscordChannelId()
	if err != nil || strings.TrimSpace(channelID) == "" {
		return
	}
	if msg.ChannelID != strings.TrimSpace(channelID) {
		return
	}

	content := strings.TrimSpace(msg.Content)
	if !strings.HasPrefix(content, "!") && !strings.HasPrefix(content, "/") {
		return
	}

	parts := strings.Fields(content)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	cmd = strings.TrimLeft(cmd, "!/")
	args := parts[1:]

	switch cmd {
	case "help", "start":
		g.sendHelp(ctx)
	case "status":
		g.sendStatus(ctx)
	case "report":
		_ = g.discordService.SendReport(ctx, g.serverService, g.inboundService)
	case "backup":
		g.sendBackup(ctx)
	case "usage":
		if len(args) == 0 {
			_ = g.discordService.SendMessage(ctx, MessagePayload{
				Content: "⚠️ Usage: `!usage <email>` or `/usage <email>`",
			})
			return
		}
		g.sendUsage(ctx, args[0])
	case "inbounds":
		g.sendInbounds(ctx)
	case "restart":
		g.restartXray(ctx)
	}
}

func (g *GatewayClient) sendHelp(ctx context.Context) {
	embed := Embed{
		Title:       "🤖 3x-ui Discord Bot Commands",
		Description: "Available commands for monitoring and managing the 3x-ui server:",
		Color:       ColorBlue,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields: []EmbedField{
			{Name: "!status", Value: "Display system load, RAM, CPU, connections, and online users", Inline: false},
			{Name: "!report", Value: "Generate a full status report (with DB backup if configured)", Inline: false},
			{Name: "!backup", Value: "Send the database backup file immediately", Inline: false},
			{Name: "!usage <email>", Value: "Query traffic usage, quota, and expiry for a client", Inline: false},
			{Name: "!inbounds", Value: "List all configured inbounds with ports and client stats", Inline: false},
			{Name: "!restart", Value: "Restart the Xray core", Inline: false},
			{Name: "!help", Value: "Display this list of available commands", Inline: false},
		},
		Footer: &EmbedFooter{Text: "3x-ui Panel"},
	}
	_ = g.discordService.SendEmbed(ctx, embed)
}

func (g *GatewayClient) sendStatus(ctx context.Context) {
	var status *service.Status
	if g.serverService != nil {
		status = g.serverService.GetStatus(nil)
	}
	if status == nil {
		status = &service.Status{}
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "3x-ui"
	}

	days := status.Uptime / 86400
	hours := (status.Uptime % 86400) / 3600

	var onlines []string
	if process := service.XrayProcess(); process != nil && process.IsRunning() {
		onlines = process.GetOnlineClients()
	}

	load1, load2, load3 := 0.0, 0.0, 0.0
	if len(status.Loads) > 0 {
		load1 = status.Loads[0]
	}
	if len(status.Loads) > 1 {
		load2 = status.Loads[1]
	}
	if len(status.Loads) > 2 {
		load3 = status.Loads[2]
	}

	embed := Embed{
		Title:       "⚡ 3x-ui Server Status",
		Description: fmt.Sprintf("Current operational metrics for **%s**", hostname),
		Color:       ColorGreen,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields: []EmbedField{
			{Name: "Panel Version", Value: config.GetPanelVersion(), Inline: true},
			{Name: "Xray State", Value: fmt.Sprintf("%s (%s)", status.Xray.Version, status.Xray.State), Inline: true},
			{Name: "Uptime", Value: fmt.Sprintf("%dd %dh", days, hours), Inline: true},
			{Name: "System Load", Value: fmt.Sprintf("%.2f, %.2f, %.2f", load1, load2, load3), Inline: true},
			{Name: "RAM", Value: fmt.Sprintf("%s / %s", common.FormatTraffic(int64(status.Mem.Current)), common.FormatTraffic(int64(status.Mem.Total))), Inline: true},
			{Name: "Online Clients", Value: strconv.Itoa(len(onlines)), Inline: true},
			{Name: "Connections", Value: fmt.Sprintf("TCP: %d | UDP: %d", status.TcpCount, status.UdpCount), Inline: true},
			{Name: "Traffic Sent", Value: common.FormatTraffic(int64(status.NetTraffic.Sent)), Inline: true},
			{Name: "Traffic Recv", Value: common.FormatTraffic(int64(status.NetTraffic.Recv)), Inline: true},
		},
		Footer: &EmbedFooter{Text: "3x-ui Status"},
	}
	_ = g.discordService.SendEmbed(ctx, embed)
}

func (g *GatewayClient) sendBackup(ctx context.Context) {
	if g.serverService == nil {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: "❌ Backup service unavailable"})
		return
	}

	dbData, err := g.serverService.GetDb()
	if err != nil || len(dbData) == 0 {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: fmt.Sprintf("❌ Failed to read database backup: %v", err)})
		return
	}

	filename := g.serverService.BackupFilename("")
	if filename == "" {
		filename = "x-ui.db"
	}

	files := []FileAttachment{
		{Filename: filename, Data: dbData},
	}

	configPath := xray.GetConfigPath()
	if configData, err := os.ReadFile(configPath); err == nil && len(configData) > 0 {
		files = append(files, FileAttachment{
			Filename: "config.json",
			Data:     configData,
		})
	}

	payload := MessagePayload{
		Embeds: []Embed{
			{
				Title:       "🗄️ Database Backup",
				Description: fmt.Sprintf("Backup archive for 3x-ui generated at `%s`", time.Now().UTC().Format(time.RFC3339)),
				Color:       ColorBlue,
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				Footer:      &EmbedFooter{Text: "3x-ui Backup"},
			},
		},
	}

	_ = g.discordService.SendMessageWithFiles(ctx, payload, files...)
}

func (g *GatewayClient) sendUsage(ctx context.Context, email string) {
	if g.inboundService == nil {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: "❌ Inbound service unavailable"})
		return
	}

	inbounds, err := g.inboundService.GetAllInbounds()
	if err != nil {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: fmt.Sprintf("❌ Failed to load inbounds: %v", err)})
		return
	}

	target := strings.ToLower(strings.TrimSpace(email))
	for _, in := range inbounds {
		for _, client := range in.ClientStats {
			if strings.ToLower(client.Email) == target {
				color := ColorGreen
				statusStr := "Active"
				if !client.Enable {
					color = ColorRed
					statusStr = "Disabled"
				}

				expireStr := "Unlimited"
				if client.ExpiryTime > 0 {
					expireStr = time.Unix(client.ExpiryTime/1000, 0).Format("2006-01-02 15:04:05")
				}

				totalLimitStr := "Unlimited"
				if client.Total > 0 {
					totalLimitStr = common.FormatTraffic(client.Total)
				}

				embed := Embed{
					Title:       fmt.Sprintf("👤 Client Usage: %s", client.Email),
					Description: fmt.Sprintf("Inbound: **%s** (Port %d)", in.Remark, in.Port),
					Color:       color,
					Timestamp:   time.Now().UTC().Format(time.RFC3339),
					Fields: []EmbedField{
						{Name: "Status", Value: statusStr, Inline: true},
						{Name: "Upload", Value: common.FormatTraffic(client.Up), Inline: true},
						{Name: "Download", Value: common.FormatTraffic(client.Down), Inline: true},
						{Name: "Total Used", Value: common.FormatTraffic(client.Up + client.Down), Inline: true},
						{Name: "Quota", Value: totalLimitStr, Inline: true},
						{Name: "Expiry Date", Value: expireStr, Inline: true},
					},
					Footer: &EmbedFooter{Text: "3x-ui Client Traffic"},
				}
				_ = g.discordService.SendEmbed(ctx, embed)
				return
			}
		}
	}

	_ = g.discordService.SendMessage(ctx, MessagePayload{
		Content: fmt.Sprintf("⚠️ Client `%s` was not found in any configured inbound.", email),
	})
}

func (g *GatewayClient) sendInbounds(ctx context.Context) {
	if g.inboundService == nil {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: "❌ Inbound service unavailable"})
		return
	}

	inbounds, err := g.inboundService.GetAllInbounds()
	if err != nil {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: fmt.Sprintf("❌ Failed to load inbounds: %v", err)})
		return
	}

	if len(inbounds) == 0 {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: "ℹ️ No inbounds configured."})
		return
	}

	var fields []EmbedField
	for _, in := range inbounds {
		state := "Active"
		if !in.Enable {
			state = "Disabled"
		}
		val := fmt.Sprintf("Protocol: `%s` | Port: `%d` | Clients: `%d` | Traffic: `↑%s ↓%s` | State: `%s`",
			in.Protocol,
			in.Port,
			len(in.ClientStats),
			common.FormatTraffic(in.Up),
			common.FormatTraffic(in.Down),
			state,
		)
		fields = append(fields, EmbedField{
			Name:   fmt.Sprintf("📍 %s", in.Remark),
			Value:  val,
			Inline: false,
		})
	}

	embed := Embed{
		Title:       "🔌 Configured Inbounds",
		Description: fmt.Sprintf("Total inbounds: **%d**", len(inbounds)),
		Color:       ColorBlue,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields:      fields,
		Footer:      &EmbedFooter{Text: "3x-ui Inbounds"},
	}
	_ = g.discordService.SendEmbed(ctx, embed)
}

func (g *GatewayClient) restartXray(ctx context.Context) {
	if g.xrayService == nil {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: "❌ Xray service unavailable"})
		return
	}

	_ = g.discordService.SendMessage(ctx, MessagePayload{Content: "🔄 Restarting Xray core..."})
	if err := g.xrayService.RestartXray(false); err != nil {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: fmt.Sprintf("❌ Failed to restart Xray: %v", err)})
	} else {
		_ = g.discordService.SendMessage(ctx, MessagePayload{Content: "✅ Xray core restarted successfully."})
	}
}
