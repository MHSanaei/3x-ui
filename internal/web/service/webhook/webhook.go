// Package webhook implements a generic HTTP webhook notification channel
// for the 3x-ui event bus. Mirrors email.EmailService's shape and conventions.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const (
	sendTimeout     = 10 * time.Second
	signatureHeader = "X-Webhook-Signature"
	eventHeader     = "X-Webhook-Event"
)

// WebhookService delivers event bus notifications as signed JSON HTTP POSTs
// to a single configured URL.
type WebhookService struct {
	settingService service.SettingService
	client         *http.Client
}

// TestResult mirrors email.SMTPTestResult so the settings UI can reuse the
// same "Test" button / stage-reporting pattern.
type TestResult struct {
	Success bool   `json:"success"`
	Stage   string `json:"stage"` // "config" | "send"
	Message string `json:"message"`
}

// Payload is the JSON body POSTed to the configured webhook URL.
type Payload struct {
	Event     eventbus.EventType `json:"event"`
	Source    string             `json:"source,omitempty"`
	Timestamp int64              `json:"timestamp"`
	Data      any                `json:"data,omitempty"`
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(settingService service.SettingService) *WebhookService {
	return &WebhookService{
		settingService: settingService,
		client: &http.Client{
			Timeout: sendTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Send gates threshold-based events (mirroring email's formatMessage), then
// builds and POSTs the payload. Returns nil without sending when the event
// is below its configured threshold — this is not an error, just "not due yet".
func (w *WebhookService) Send(e eventbus.Event) error {
	switch e.Type {
	case eventbus.EventCPUHigh, eventbus.EventMemoryHigh:
		data, ok := e.Data.(*eventbus.SystemMetricData)
		if !ok {
			return nil
		}
		var threshold int
		var err error
		if e.Type == eventbus.EventCPUHigh {
			threshold, err = w.settingService.GetWebhookCpu()
		} else {
			threshold, err = w.settingService.GetWebhookMemory()
		}
		if err != nil || threshold <= 0 || data.Percent <= float64(threshold) {
			return nil
		}
	}

	url, err := w.settingService.GetWebhookURL()
	if err != nil || url == "" {
		return fmt.Errorf("webhook url not configured")
	}
	secret, _ := w.settingService.GetWebhookSecret()

	return w.deliver(url, secret, e.Type, e.Source, e.Data)
}

func (w *WebhookService) deliver(url, secret string, event eventbus.EventType, source string, data any) error {
	payload := Payload{
		Event:     event,
		Source:    source,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(eventHeader, string(event))
	if secret != "" {
		req.Header.Set(signatureHeader, sign(secret, body))
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

// TestConnection sends a synthetic "test" event, mirroring
// EmailService.TestConnection so the settings UI can offer a "Test" button.
func (w *WebhookService) TestConnection() TestResult {
	url, err := w.settingService.GetWebhookURL()
	if err != nil || url == "" {
		return TestResult{false, "config", "webhookUrlNotConfigured"}
	}
	secret, _ := w.settingService.GetWebhookSecret()

	if err := w.deliver(url, secret, "test", "", map[string]string{
		"message": "3x-ui webhook test notification",
	}); err != nil {
		return TestResult{false, "send", err.Error()}
	}
	return TestResult{true, "send", "webhookTestSuccess"}
}
