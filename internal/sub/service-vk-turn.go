package sub

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/vkturn"
)

func (s *SubService) genVKTurnLinks(inbound *model.Inbound, email string) string {
	cfg, err := vkturn.SettingsFromInbound(inbound)
	if err != nil || !cfg.Enabled {
		return ""
	}
	client, ok := s.clientForLink(inbound, email)
	if !ok || client.PrivateKey == "" {
		return ""
	}
	settings := s.linkSettings(inbound)
	serverPrivate, _ := settings["secretKey"].(string)
	serverPublic, err := wgutil.PublicKeyFromPrivate(serverPrivate)
	if err != nil {
		return ""
	}
	address := ""
	for _, cidr := range client.AllowedIPs {
		ip, _, err := net.ParseCIDR(cidr)
		if err == nil && ip.To4() != nil {
			address = cidr
			break
		}
	}
	if address == "" {
		return ""
	}
	dns, _ := settings["dns"].(string)
	if strings.TrimSpace(dns) == "" {
		dns = "1.1.1.1, 1.0.0.1"
	}
	profile := map[string]any{
		"privateKey": client.PrivateKey, "peerPublicKey": serverPublic,
		"presharedKey": client.PreSharedKey, "tunnelAddress": address,
		"vkLink": strings.TrimSpace(cfg.CallLink), "dnsServers": dns,
		"numConnections": 30, "useDTLS": true, "useSrtp": true,
		"useUDP": false, "useWrap": false, "useWrapA": false,
		"useWrapS": false, "useCsqtt": false, "wrapKeyHex": "",
	}
	endpoints := s.advertisedEndpoints(inbound)
	links := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		profile["peerAddress"] = joinHostPort(endpoint.Address, cfg.Port)
		payload, err := json.Marshal(map[string]any{"version": 1, "type": "connection", "settings": profile})
		if err != nil {
			return ""
		}
		query := url.Values{"data": {base64.StdEncoding.EncodeToString(payload)}}
		links = append(links, "vkturnproxy://import?"+query.Encode())
	}
	return strings.Join(links, "\n")
}
