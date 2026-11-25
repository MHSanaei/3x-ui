package service

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/util/cronspec"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func (s *SettingService) GetWechatEnable() (bool, error) { return s.getBool("wechatEnable") }
func (s *SettingService) GetWechatMode() (string, error) { return s.getString("wechatMode") }
func (s *SettingService) GetWechatWebhookURL() (string, error) {
	return s.getString("wechatWebhookURL")
}
func (s *SettingService) GetWechatToken() (string, error) { return s.getString("wechatToken") }
func (s *SettingService) GetWechatLang() (string, error)  { return s.getString("wechatLang") }
func (s *SettingService) GetWechatReportEnable() (bool, error) {
	return s.getBool("wechatReportEnable")
}
func (s *SettingService) GetWechatRunTime() (string, error) { return s.getString("wechatRunTime") }
func (s *SettingService) GetWechatEnabledEvents() (string, error) {
	return s.getString("wechatEnabledEvents")
}
func (s *SettingService) GetWechatCpu() (int, error)    { return s.getInt("wechatCpu") }
func (s *SettingService) GetWechatMemory() (int, error) { return s.getInt("wechatMemory") }

type WechatDeliverySettings struct {
	Enabled    bool
	Mode       string
	WebhookURL string
	Token      string
}

// GetWechatDeliverySettings reads a coherent destination and credential snapshot.
func (s *SettingService) GetWechatDeliverySettings() (WechatDeliverySettings, error) {
	keys := []string{"wechatEnable", "wechatMode", "wechatWebhookURL", "wechatToken"}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = defaultValueMap[key]
	}
	var rows []model.Setting
	if err := database.GetDB().Where("key IN ?", keys).Find(&rows).Error; err != nil {
		return WechatDeliverySettings{}, err
	}
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	enabled, err := strconv.ParseBool(effectiveSettingValue("wechatEnable", values["wechatEnable"]))
	if err != nil {
		return WechatDeliverySettings{}, err
	}
	return WechatDeliverySettings{
		Enabled: enabled, Mode: values["wechatMode"], WebhookURL: values["wechatWebhookURL"], Token: values["wechatToken"],
	}, nil
}

// ValidateWechatDestination leaves DNS checks to the guarded dial, so saving
// settings cannot contact the receiver or leak a credential in an error.
func ValidateWechatDestination(mode, raw string) (string, error) {
	if mode != "wecom" && mode != "webhook" {
		return "", errors.New("WeChat notification mode is invalid")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 4096 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return "", errors.New("WeChat webhook URL is invalid")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.Contains(host, "%") {
		return "", errors.New("WeChat webhook URL is invalid")
	}
	if ip := net.ParseIP(host); ip != nil && netsafe.IsBlockedIP(ip) {
		return "", errors.New("WeChat webhook URL is invalid")
	}
	if mode == "wecom" && (u.Host != "qyapi.weixin.qq.com" && u.Host != "qyapi.weixin.qq.com:443" || u.Path != "/cgi-bin/webhook/send" || u.RawPath != "" || u.Query().Get("key") == "" || len(u.Query()) != 1 || len(u.Query()["key"]) != 1) {
		return "", errors.New("WeChat webhook URL is invalid")
	}
	return u.String(), nil
}

func validateWechatSettings(settings *entity.AllSetting) error {
	if settings.WechatMode == "" {
		settings.WechatMode = "wecom"
	}
	endpoint, err := ValidateWechatDestination(settings.WechatMode, settings.WechatWebhookURL)
	if err != nil {
		return err
	}
	if settings.WechatEnable && endpoint == "" {
		return errors.New("WeChat webhook URL is required")
	}
	settings.WechatWebhookURL = endpoint
	if len(settings.WechatToken) > 4096 || strings.ContainsAny(settings.WechatToken, "\r\n") {
		return errors.New("WeChat bearer token is invalid")
	}
	if settings.WechatRunTime == "" {
		settings.WechatRunTime = "@daily"
	}
	if _, err := cronspec.Parse(settings.WechatRunTime); err != nil {
		return errors.New("WeChat report schedule is invalid")
	}
	for value := range strings.SplitSeq(settings.WechatEnabledEvents, ",") {
		switch eventbus.EventType(strings.TrimSpace(value)) {
		case "", eventbus.EventLoginAttempt, eventbus.EventCPUHigh, eventbus.EventMemoryHigh,
			eventbus.EventNodeDown, eventbus.EventNodeUp, eventbus.EventXrayCrash,
			eventbus.EventOutboundDown, eventbus.EventOutboundUp:
		default:
			return errors.New("WeChat notification event selection is invalid")
		}
	}
	if settings.WechatLang == "" {
		settings.WechatLang = "en-US"
	}
	return nil
}
