package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	happCryptoEndpoint  = "https://crypto.happ.su/api-v2.php"
	happProviderTimeout = 10 * time.Second
	happMaxResponseSize = 64 << 10
)

// ErrHappLinkUnavailable deliberately carries no provider or subscription detail.
var ErrHappLinkUnavailable = errors.New("happ link unavailable")

type HappLinkResult struct {
	EncryptedLink string `json:"encryptedLink" example:"happ://crypt5/example"`
}

type HappLinkGenerator interface {
	Generate(context.Context, int, string) (HappLinkResult, error)
}

// HappService generates one encrypted link per action and does not retain results.
type HappService struct {
	clientService  *ClientService
	settingService *SettingService
	endpoint       string
	newHTTPClient  func(time.Duration) *http.Client
}

func NewHappService(clientService *ClientService, settingService *SettingService) *HappService {
	return &HappService{
		clientService:  clientService,
		settingService: settingService,
		endpoint:       happCryptoEndpoint,
		newHTTPClient:  settingService.NewProxiedHTTPClient,
	}
}

func (s *HappService) Generate(ctx context.Context, clientID int, host string) (HappLinkResult, error) {
	started := time.Now()
	correlationID := uuid.NewString()
	// Operator consent is the first boundary: do not build the source URL or HTTP client while disabled.
	if reason := s.gateFailureReason(); reason != "" {
		return HappLinkResult{}, s.fail(clientID, reason, 0, started, correlationID, "generation unavailable", "", "")
	}
	source, client, reason := s.currentSource(clientID, host)
	if reason != "" {
		return HappLinkResult{}, s.fail(clientID, reason, 0, started, correlationID, "source unavailable", "", "")
	}

	body, err := json.Marshal(struct {
		URL string `json:"url"`
	}{URL: source})
	if err != nil {
		return HappLinkResult{}, s.fail(clientID, "request_encode", 0, started, correlationID, "request encoding failed", source, client.SubID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return HappLinkResult{}, s.fail(clientID, "request_create", 0, started, correlationID, "request creation failed", source, client.SubID)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	baseClient := s.newHTTPClient(happProviderTimeout)
	if baseClient == nil {
		return HappLinkResult{}, s.fail(clientID, "transport", 0, started, correlationID, "request failed", source, client.SubID)
	}
	// Do not mutate the shared proxied client: other panel requests may use it concurrently.
	httpClient := *baseClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := httpClient.Do(req)
	if err != nil {
		return HappLinkResult{}, s.fail(clientID, "transport", 0, started, correlationID, "request failed", source, client.SubID)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return HappLinkResult{}, s.fail(clientID, "http_status", resp.StatusCode, started, correlationID, "provider returned non-success status", source, client.SubID)
	}

	responseBody, err := readHappResponse(resp.Body)
	if err != nil {
		reason := "response_read"
		if errors.Is(err, errHappResponseTooLarge) {
			reason = "response_too_large"
		}
		return HappLinkResult{}, s.fail(clientID, reason, resp.StatusCode, started, correlationID, "invalid provider response", source, client.SubID)
	}
	link, reason := parseHappResponse(responseBody)
	if reason != "" {
		return HappLinkResult{}, s.fail(clientID, reason, resp.StatusCode, started, correlationID, "invalid provider response", source, client.SubID)
	}

	currentSource, _, currentReason := s.currentSource(clientID, host)
	if currentReason != "" || currentSource != source {
		return HappLinkResult{}, s.fail(clientID, "source_changed", resp.StatusCode, started, correlationID, "source changed before response", source, client.SubID)
	}
	// A sent request cannot be recalled, so revoked consent discards the provider result before return.
	if reason := s.gateFailureReason(); reason != "" {
		return HappLinkResult{}, s.fail(clientID, reason, resp.StatusCode, started, correlationID, "generation unavailable", "", "")
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

var errHappResponseTooLarge = errors.New("happ response exceeds size limit")

var happSensitiveDetailToken = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://\S+|(?:token|secret|password|passwd|credential|authorization|bearer|api[_-]?key)\s*(?:=|:)\s*\S+)`)

func readHappResponse(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, happMaxResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > happMaxResponseSize {
		return nil, errHappResponseTooLarge
	}
	return body, nil
}

func parseHappResponse(body []byte) (string, string) {
	var response struct {
		EncryptedLink json.RawMessage `json:"encrypted_link"`
		Error         json.RawMessage `json:"error"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil {
		return "", "response_json"
	}
	if start == nil {
		return "", "response_shape"
	}
	delimiter, ok := start.(json.Delim)
	if !ok || delimiter != '{' {
		return "", "response_json"
	}
	seenLink := false
	seenError := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", "response_json"
		}
		name, ok := key.(string)
		if !ok {
			return "", "response_json"
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "", "response_json"
		}
		switch name {
		case "encrypted_link":
			if seenLink {
				return "", "response_shape"
			}
			seenLink = true
			response.EncryptedLink = value
		case "error":
			if seenError {
				return "", "response_shape"
			}
			seenError = true
			response.Error = value
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return "", "response_json"
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", "response_json"
	}
	hasLink := len(response.EncryptedLink) != 0
	hasError := len(response.Error) != 0
	if hasLink == hasError {
		return "", "response_shape"
	}
	if hasError {
		var providerError string
		if err := json.Unmarshal(response.Error, &providerError); err != nil || providerError == "" {
			return "", "response_shape"
		}
		return "", "provider_error"
	}
	var link string
	if err := json.Unmarshal(response.EncryptedLink, &link); err != nil {
		return "", "response_shape"
	}
	if !validHappLink(link) {
		return "", "link_invalid"
	}
	return link, ""
}

func validHappLink(link string) bool {
	if !strings.HasPrefix(link, "happ://crypt5/") || len(link) == len("happ://crypt5/") {
		return false
	}
	if strings.IndexFunc(link, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return false
	}
	parsed, err := url.ParseRequestURI(link)
	return err == nil && parsed.Scheme == "happ" && parsed.Host == "crypt5"
}

func (s *HappService) fail(clientID int, reason string, status int, started time.Time, correlationID, detail, source, subID string) error {
	statusPart := ""
	if status != 0 {
		statusPart = " status=" + strconv.Itoa(status)
	}
	logger.Warningf("component=happ_link operation=generate outcome=failure client_id=%d reason=%s%s elapsed_ms=%d correlation_id=%s detail=%s",
		clientID, reason, statusPart, time.Since(started).Milliseconds(), correlationID, sanitizeHappDetail(detail, source, subID))
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
