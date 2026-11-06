package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var (
	ErrConfiguration = errors.New("wechat notification configuration is invalid")
	ErrDelivery      = errors.New("wechat notification delivery failed")
	ErrTooLong       = errors.New("wechat notification exceeds the text limit")
)

type Envelope struct {
	Version    int    `json:"version"`
	ID         string `json:"id"`
	Type       string `json:"type"`
	Event      string `json:"event"`
	Hostname   string `json:"hostname"`
	OccurredAt string `json:"occurred_at"`
	Text       string `json:"text"`
}

type Service struct {
	settings     service.SettingService
	httpClient   *http.Client
	sendGate     chan struct{}
	lastSend     time.Time
	sendInterval time.Duration
	retryWait    func(context.Context, time.Duration) error
}

func NewService(settings service.SettingService) *Service {
	return &Service{
		settings: settings, sendInterval: 3100 * time.Millisecond, retryWait: waitContext,
		sendGate: make(chan struct{}, 1),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				Proxy: nil, DialContext: netsafe.SSRFGuardedDialContext,
				TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second,
				IdleConnTimeout: 30 * time.Second,
			},
		},
	}
}

func (s *Service) client() *http.Client {
	client := s.httpClient
	if client == nil {
		client = NewService(s.settings).httpClient
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if copy.Timeout <= 0 || copy.Timeout > 10*time.Second {
		copy.Timeout = 10 * time.Second
	}
	return &copy
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(max(0, duration))
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ErrDelivery
	}
}

func (s *Service) configuration() (string, string, string, error) {
	settings, err := s.settings.GetWechatDeliverySettings()
	if err != nil || !settings.Enabled {
		return "", "", "", ErrConfiguration
	}
	if strings.TrimSpace(settings.WebhookURL) == "" {
		return "", "", "", ErrConfiguration
	}
	endpoint, err := service.ValidateWechatDestination(settings.Mode, settings.WebhookURL)
	if err != nil {
		return "", "", "", ErrConfiguration
	}
	if len(settings.Token) > 4096 || strings.ContainsAny(settings.Token, "\r\n") {
		return "", "", "", ErrConfiguration
	}
	return settings.Mode, endpoint, strings.TrimSpace(settings.Token), nil
}

func (s *Service) send(ctx context.Context, kind, event, text string, occurredAt time.Time) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = netsafe.ContextWithAllowPrivate(ctx, false)
	mode, endpoint, token, err := s.configuration()
	if err != nil {
		return err
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "3x-ui"
	}
	text = "3x-ui · " + plain(hostname) + "\n" + text
	if !utf8.ValidString(text) || len(text) > 20480 {
		return ErrTooLong
	}
	if mode == "webhook" {
		body, err := json.Marshal(Envelope{
			Version: 1, ID: uuid.NewString(), Type: kind, Event: event, Hostname: hostname,
			OccurredAt: occurredAt.UTC().Format(time.RFC3339), Text: text,
		})
		if err != nil {
			return ErrDelivery
		}
		return s.sendWebhook(ctx, endpoint, token, body)
	}
	pieces, err := splitText(text)
	if err != nil {
		return err
	}
	select {
	case s.sendGate <- struct{}{}:
		defer func() { <-s.sendGate }()
	case <-ctx.Done():
		return ErrDelivery
	}
	for _, piece := range pieces {
		if err := s.retryWait(ctx, s.sendInterval-time.Since(s.lastSend)); err != nil {
			return ErrDelivery
		}
		body, err := json.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": piece}})
		if err != nil {
			return ErrDelivery
		}
		s.lastSend = time.Now()
		_, response, err := s.post(ctx, endpoint, "", body)
		if err != nil {
			return ErrDelivery
		}
		var result struct {
			Errcode *int `json:"errcode"`
		}
		if json.Unmarshal(response, &result) != nil || result.Errcode == nil || *result.Errcode != 0 {
			return ErrDelivery
		}
	}
	return nil
}

func splitText(text string) ([]string, error) {
	var result []string
	for len(text) > 2048 {
		cut := 2048
		for !utf8.RuneStart(text[cut]) {
			cut--
		}
		result = append(result, text[:cut])
		text = text[cut:]
	}
	if text != "" {
		result = append(result, text)
	}
	if len(result) > 10 {
		return nil, ErrTooLong
	}
	return result, nil
}

func (s *Service) sendWebhook(ctx context.Context, endpoint, token string, body []byte) error {
	for attempt := range 2 {
		status, _, err := s.post(ctx, endpoint, token, body)
		if err == nil {
			return nil
		}
		if attempt == 1 || (status != 0 && status < 500) || ctx.Err() != nil {
			return ErrDelivery
		}
		if s.retryWait(ctx, time.Second) != nil {
			return ErrDelivery
		}
	}
	return ErrDelivery
}

func (s *Service) post(ctx context.Context, endpoint, token string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, ErrDelivery
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "3x-ui-WeChat/1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := s.client().Do(req)
	if err != nil {
		return 0, nil, ErrDelivery
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, nil, ErrDelivery
	}
	return response.StatusCode, data, nil
}

func (s *Service) translator() func(string, ...string) string {
	lang, err := s.settings.GetWechatLang()
	if err != nil || lang == "" {
		lang = "en-US"
	}
	return func(key string, params ...string) string { return locale.I18nForLang(lang, key, params...) }
}

func (s *Service) SendTest() error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	tr := s.translator()
	return s.send(ctx, "test", "test", tr("wechat.test.title")+"\n"+tr("wechat.test.body"), time.Now())
}
