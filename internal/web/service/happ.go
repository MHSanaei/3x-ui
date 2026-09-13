package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

var (
	// Failures deliberately carry no subscription details.
	ErrHappLinkUnavailable = errors.New("happ link unavailable")
	ErrHappSourceTooLong   = errors.New("happ subscription source exceeds 8192 bytes")
)

type HappLinkResult struct {
	EncryptedLink string `json:"encryptedLink" example:"happ://crypt5/example"`
}

type HappLinkGenerator interface {
	Generate(context.Context, int, string) (HappLinkResult, error)
}

// HappService generates one local encrypted link per action and does not retain results.
type HappService struct {
	clientService  *ClientService
	settingService *SettingService
	encrypt        func(string) (string, error)
}

func NewHappService(clientService *ClientService, settingService *SettingService) *HappService {
	return &HappService{
		clientService:  clientService,
		settingService: settingService,
		encrypt:        encryptHappLink,
	}
}

func (s *HappService) Generate(ctx context.Context, clientID int, host string) (HappLinkResult, error) {
	started := time.Now()
	correlationID := uuid.NewString()
	// Check the operator gate before constructing a subscription URL or encrypting it.
	if reason := s.gateFailureReason(); reason != "" {
		return HappLinkResult{}, s.fail(clientID, reason, started, correlationID, "generation unavailable", "", "")
	}
	if ctx.Err() != nil {
		return HappLinkResult{}, s.fail(clientID, "request_cancelled", started, correlationID, "request cancelled", "", "")
	}
	source, client, reason := s.currentSource(clientID, host)
	if reason != "" {
		return HappLinkResult{}, s.fail(clientID, reason, started, correlationID, "source unavailable", "", "")
	}
	if s.encrypt == nil {
		return HappLinkResult{}, s.fail(clientID, "service_unavailable", started, correlationID, "encryption unavailable", "", "")
	}
	link, err := s.encrypt(source)
	if err != nil {
		if errors.Is(err, ErrHappSourceTooLong) {
			_ = s.fail(clientID, "source_too_long", started, correlationID, "source exceeds application byte limit", "", "")
			return HappLinkResult{}, ErrHappSourceTooLong
		}
		return HappLinkResult{}, s.fail(clientID, "encryption", started, correlationID, err.Error(), source, client.SubID)
	}
	if ctx.Err() != nil {
		return HappLinkResult{}, s.fail(clientID, "request_cancelled", started, correlationID, "request cancelled", "", "")
	}
	currentSource, _, currentReason := s.currentSource(clientID, host)
	if currentReason != "" || currentSource != source {
		return HappLinkResult{}, s.fail(clientID, "source_changed", started, correlationID, "source changed before response", "", "")
	}
	// Local work can still overlap a settings change; discard results after the gate is disabled.
	if reason := s.gateFailureReason(); reason != "" {
		return HappLinkResult{}, s.fail(clientID, reason, started, correlationID, "generation unavailable", "", "")
	}
	return HappLinkResult{EncryptedLink: link}, nil
}

func (s *HappService) gateFailureReason() string {
	if s.settingService == nil {
		return "service_unavailable"
	}
	enabled, err := s.settingService.GetHappLinkEnable()
	if err != nil {
		return "settings_unavailable"
	}
	if !enabled {
		return "integration_disabled"
	}
	return ""
}

func (s *HappService) currentSource(clientID int, host string) (string, *model.ClientRecord, string) {
	if s.clientService == nil || s.settingService == nil {
		return "", nil, "service_unavailable"
	}
	client, err := s.clientService.GetByID(clientID)
	if err != nil {
		return "", nil, "client_unavailable"
	}
	settings, err := s.settingService.GetDefaultSettings(host)
	if err != nil {
		return "", client, "settings_unavailable"
	}
	values, ok := settings.(map[string]any)
	if !ok {
		return "", client, "settings_unavailable"
	}
	subEnable, enabled := values["subEnable"].(bool)
	subURI, hasURI := values["subURI"].(string)
	if !enabled || !subEnable || !hasURI || subURI == "" || client.SubID == "" {
		return "", client, "source_unavailable"
	}
	return subURI + client.SubID, client, ""
}

var happSensitiveDetailToken = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://\S+|(?:token|secret|password|passwd|credential|authorization|bearer|api[_-]?key|cookie|session)\s*(?:=|:)\s*\S+)`)

func (s *HappService) fail(clientID int, reason string, started time.Time, correlationID, detail, source, subID string) error {
	logger.Warningf("component=happ_link operation=generate outcome=failure client_id=%d reason=%s elapsed_ms=%d correlation_id=%s detail=%s",
		clientID, reason, time.Since(started).Milliseconds(), correlationID, sanitizeHappDetail(detail, source, subID))
	return ErrHappLinkUnavailable
}

func sanitizeHappDetail(detail, source, subID string) string {
	if source != "" {
		detail = strings.ReplaceAll(detail, source, "[redacted]")
	}
	if subID != "" {
		detail = strings.ReplaceAll(detail, subID, "[redacted]")
	}
	detail = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, detail)
	detail = happSensitiveDetailToken.ReplaceAllString(detail, "[redacted]")
	runes := []rune(detail)
	if len(runes) > 160 {
		detail = string(runes[:160])
	}
	return detail
}
