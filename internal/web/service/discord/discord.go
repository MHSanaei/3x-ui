package discord

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const (
	defaultDiscordBaseURL = "https://discord.com/api/v10"
	discordUserAgent      = "DiscordBot (https://github.com/mhsanaei/3x-ui, 3.x)"

	ColorGreen  = 0x2ECC71
	ColorRed    = 0xE74C3C
	ColorOrange = 0xF39C12
)

// MessagePayload represents the Discord create message payload.
type MessagePayload struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

// Embed represents a Discord embed object.
type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
}

// EmbedField represents a field in a Discord embed.
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// EmbedFooter represents a footer in a Discord embed.
type EmbedFooter struct {
	Text string `json:"text"`
}

// DiscordService manages communication with the Discord API.
type DiscordService struct {
	settingService service.SettingService
	httpClient     *http.Client
	baseURL        string
}

// NewDiscordService creates a new DiscordService.
func NewDiscordService(settingService service.SettingService) *DiscordService {
	return &DiscordService{
		settingService: settingService,
		baseURL:        defaultDiscordBaseURL,
	}
}

// SetHTTPClient sets a custom HTTP client (useful for unit testing).
func (s *DiscordService) SetHTTPClient(client *http.Client) {
	s.httpClient = client
}

// SetBaseURL sets a custom base URL for the Discord API (useful for testing with httptest).
func (s *DiscordService) SetBaseURL(url string) {
	s.baseURL = strings.TrimRight(url, "/")
}

func (s *DiscordService) getClient() *http.Client {
	if s.httpClient != nil {
		return s.httpClient
	}
	return s.settingService.NewProxiedHTTPClient(10 * time.Second)
}

// SendMessage sends a Discord message payload to the configured channel.
func (s *DiscordService) SendMessage(payload MessagePayload) error {
	token, err := s.settingService.GetDiscordBotToken()
	if err != nil || strings.TrimSpace(token) == "" {
		return errors.New("discord bot token is not configured")
	}
	channelID, err := s.settingService.GetDiscordChannelId()
	if err != nil || strings.TrimSpace(channelID) == "" {
		return errors.New("discord channel id is not configured")
	}

	cleanToken := strings.TrimSpace(token)
	cleanToken = strings.TrimPrefix(cleanToken, "Bot ")
	cleanToken = strings.TrimSpace(cleanToken)
	if cleanToken == "" {
		return errors.New("discord bot token is not configured")
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	baseURL := s.baseURL
	if baseURL == "" {
		baseURL = defaultDiscordBaseURL
	}
	endpoint := fmt.Sprintf("%s/channels/%s/messages", baseURL, strings.TrimSpace(channelID))

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create discord request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+cleanToken)
	req.Header.Set("User-Agent", discordUserAgent)

	resp, err := s.getClient().Do(req)
	if err != nil {
		return fmt.Errorf("discord request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyStr := string(respBody)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusBadRequest:
		return fmt.Errorf("discord bad request (400): %s", bodyStr)
	case http.StatusUnauthorized:
		return errors.New("discord unauthorized (401): invalid bot token")
	case http.StatusForbidden:
		return errors.New("discord forbidden (403): bot lacks permissions for channel")
	case http.StatusNotFound:
		return errors.New("discord not found (404): channel not found")
	case http.StatusTooManyRequests:
		return fmt.Errorf("discord rate limited (429): %s", bodyStr)
	default:
		return fmt.Errorf("discord API error (%d): %s", resp.StatusCode, bodyStr)
	}
}

// SendEmbed is a helper to send an embed payload.
func (s *DiscordService) SendEmbed(embed Embed) error {
	return s.SendMessage(MessagePayload{
		Embeds: []Embed{embed},
	})
}

// SendTest sends a test embed to verify Discord bot configuration.
func (s *DiscordService) SendTest() error {
	now := time.Now().UTC().Format(time.RFC3339)
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "3x-ui"
	}
	embed := Embed{
		Title:       "3x-ui Discord Notification Test",
		Description: "This is a test notification confirming that Discord notifications are configured correctly.",
		Color:       ColorGreen,
		Timestamp:   now,
		Fields: []EmbedField{
			{Name: "Host", Value: hostname, Inline: true},
		},
		Footer: &EmbedFooter{
			Text: "3x-ui Panel",
		},
	}
	return s.SendEmbed(embed)
}
