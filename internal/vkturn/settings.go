package vkturn

import (
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

var callLinkPattern = regexp.MustCompile(`^https://(?:vk\.ru|vk\.com)/call/join/[A-Za-z0-9_-]+/?$`)

type Settings struct {
	Enabled  bool   `json:"enabled"`
	CallLink string `json:"callLink"`
	Port     int    `json:"port"`
}

type Instance struct {
	ID      int
	Listen  string
	Connect string
}

func SettingsFromInbound(inbound *model.Inbound) (Settings, error) {
	if inbound.Protocol != model.WireGuard || strings.TrimSpace(inbound.Settings) == "" {
		return Settings{}, nil
	}
	var settings struct {
		VKTurn Settings `json:"vkTurn"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return Settings{}, err
	}
	cfg := settings.VKTurn
	if !cfg.Enabled {
		return cfg, nil
	}
	cfg.CallLink = strings.TrimSpace(cfg.CallLink)
	if !callLinkPattern.MatchString(cfg.CallLink) {
		return Settings{}, errors.New("VK call link must be an HTTPS vk.ru or vk.com call/join URL")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return Settings{}, errors.New("VK TURN UDP port must be between 1 and 65535")
	}
	if cfg.Port == inbound.Port {
		return Settings{}, errors.New("VK TURN UDP port must differ from the WireGuard port")
	}
	return cfg, nil
}

func InstanceFromInbound(inbound *model.Inbound) (Instance, bool) {
	cfg, err := SettingsFromInbound(inbound)
	if err != nil || !inbound.Enable || !cfg.Enabled {
		return Instance{}, false
	}
	host := inbound.Listen
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return Instance{ID: inbound.Id, Listen: net.JoinHostPort("0.0.0.0", strconv.Itoa(cfg.Port)), Connect: net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(inbound.Port))}, true
}
