package tuic

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type TuicServerSettings struct {
	Certificate           string   `json:"certificate"`
	PrivateKey            string   `json:"private_key"`
	CongestionControl     string   `json:"congestion_control"`
	ALPN                  []string `json:"alpn"`
	UDPRelayMode          string   `json:"udp_relay_mode"`
	ZeroRTTHandshake      bool     `json:"zero_rtt_handshake"`
	LogLevel              string   `json:"log_level"`
	MaxIdleTime           int      `json:"max_idle_time"`
	AuthenticationTimeout int      `json:"authentication_timeout"`
	MaxUdpRelayPacketSize int      `json:"max_udp_relay_packet_size"`
	SNI                   string   `json:"sni,omitempty"`
}

type TuicClientSettings struct {
	TrafficID int    `json:"-"`
	UUID      string `json:"uuid"`
	Password  string `json:"password"`
	Email     string `json:"email"`
}

type Instance struct {
	Id                    int
	Tag                   string
	Listen                string
	Port                  int
	Certificate           string
	PrivateKey            string
	CongestionControl     string // Applied to new native QUIC connections and exported to client profiles.
	ALPN                  []string
	UDPRelayMode          string // Default mode exported to client links; the listener accepts both modes.
	ZeroRTTHandshake      bool
	LogLevel              string
	MaxIdleTime           int
	AuthenticationTimeout int
	MaxUdpRelayPacketSize int
	SNI                   string // Exported to client links; it doesn't change the native listener certificate.
	Clients               []TuicClientSettings
}

func (inst Instance) BindTo() string {
	return net.JoinHostPort(inst.Listen, strconv.Itoa(inst.Port))
}

func (inst Instance) StructuralFingerprint() string {
	parts := []string{
		inst.BindTo(),
		fingerprintMaterial(inst.Certificate),
		fingerprintMaterial(inst.PrivateKey),
		strings.Join(inst.ALPN, ","),
		strconv.FormatBool(inst.ZeroRTTHandshake),
		strconv.Itoa(inst.MaxIdleTime),
		strconv.Itoa(inst.AuthenticationTimeout),
		strconv.Itoa(inst.MaxUdpRelayPacketSize),
	}
	return strings.Join(parts, "|")
}

func fingerprintMaterial(value string) string {
	material := []byte(value)
	if value != "" && !strings.Contains(value, "-----BEGIN ") {
		if file, err := os.ReadFile(value); err == nil {
			material = file
		}
	}
	return fmt.Sprintf("%x", sha256.Sum256(material))
}

func (inst Instance) UsersFingerprint() string {
	pairs := make([]string, 0, len(inst.Clients))
	for _, c := range inst.Clients {
		pairs = append(pairs, fmt.Sprintf("%d:%s=%s:%s", c.TrafficID, c.Email, c.UUID, c.Password))
	}
	slices.Sort(pairs)
	return strings.Join(pairs, "|")
}

func (inst Instance) FullFingerprint() string {
	return inst.StructuralFingerprint() + "#" + inst.UsersFingerprint()
}

func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || ib.Protocol != model.TUIC {
		return Instance{}, false
	}

	var parsed struct {
		Certificate           string   `json:"certificate"`
		PrivateKey            string   `json:"private_key"`
		CongestionControl     string   `json:"congestion_control"`
		ALPN                  []string `json:"alpn"`
		UDPRelayMode          string   `json:"udp_relay_mode"`
		ZeroRTTHandshake      *bool    `json:"zero_rtt_handshake"`
		LogLevel              string   `json:"log_level"`
		MaxIdleTime           int      `json:"max_idle_time"`
		AuthenticationTimeout int      `json:"authentication_timeout"`
		MaxUdpRelayPacketSize int      `json:"max_udp_relay_packet_size"`
		SNI                   string   `json:"sni"`
		Server                *struct {
			Certificate           string   `json:"certificate"`
			PrivateKey            string   `json:"private_key"`
			CongestionControl     string   `json:"congestion_control"`
			ALPN                  []string `json:"alpn"`
			UDPRelayMode          string   `json:"udp_relay_mode"`
			ZeroRTTHandshake      *bool    `json:"zero_rtt_handshake"`
			LogLevel              string   `json:"log_level"`
			MaxIdleTime           int      `json:"max_idle_time"`
			AuthenticationTimeout int      `json:"authentication_timeout"`
			MaxUdpRelayPacketSize int      `json:"max_udp_relay_packet_size"`
			SNI                   string   `json:"sni"`
		} `json:"server"`
		Clients []struct {
			TrafficID  int    `json:"traffic_id"`
			UUID       string `json:"uuid"`
			ID         string `json:"id"`
			Password   string `json:"password"`
			Email      string `json:"email"`
			Enable     *bool  `json:"enable"`
			TotalGB    int64  `json:"totalGB"`
			ExpiryTime int64  `json:"expiryTime"`
		} `json:"clients"`
	}

	if ib.Settings != "" {
		if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
			return Instance{}, false
		}
	}

	cert := parsed.Certificate
	key := parsed.PrivateKey
	cc := parsed.CongestionControl
	alpn := parsed.ALPN
	udpRelayMode := parsed.UDPRelayMode
	zeroRtt := true
	if parsed.ZeroRTTHandshake != nil {
		zeroRtt = *parsed.ZeroRTTHandshake
	}
	logLevel := parsed.LogLevel
	maxIdle := parsed.MaxIdleTime
	authTimeout := parsed.AuthenticationTimeout
	maxPacketSize := parsed.MaxUdpRelayPacketSize
	sni := parsed.SNI

	if parsed.Server != nil {
		if parsed.Server.Certificate != "" {
			cert = parsed.Server.Certificate
		}
		if parsed.Server.PrivateKey != "" {
			key = parsed.Server.PrivateKey
		}
		if parsed.Server.CongestionControl != "" {
			cc = parsed.Server.CongestionControl
		}
		if len(parsed.Server.ALPN) > 0 {
			alpn = parsed.Server.ALPN
		}
		if parsed.Server.UDPRelayMode != "" {
			udpRelayMode = parsed.Server.UDPRelayMode
		}
		if parsed.Server.ZeroRTTHandshake != nil {
			zeroRtt = *parsed.Server.ZeroRTTHandshake
		}
		if parsed.Server.LogLevel != "" {
			logLevel = parsed.Server.LogLevel
		}
		if parsed.Server.MaxIdleTime > 0 {
			maxIdle = parsed.Server.MaxIdleTime
		}
		if parsed.Server.AuthenticationTimeout > 0 {
			authTimeout = parsed.Server.AuthenticationTimeout
		}
		if parsed.Server.MaxUdpRelayPacketSize > 0 {
			maxPacketSize = parsed.Server.MaxUdpRelayPacketSize
		}
		if parsed.Server.SNI != "" {
			sni = parsed.Server.SNI
		}
	}

	if normalized, err := NormalizeCongestionControl(cc); err == nil {
		cc = normalized
	} else {
		cc = "new_reno"
	}
	if len(alpn) == 0 {
		alpn = []string{"h3", "spdy/3.1"}
	}
	if udpRelayMode == "" {
		udpRelayMode = "native"
	}
	if logLevel == "" {
		logLevel = "info"
	}
	if maxIdle <= 0 {
		maxIdle = 15
	}
	if authTimeout <= 0 {
		authTimeout = 3
	}
	if maxPacketSize <= 0 {
		maxPacketSize = 1500
	} else if maxPacketSize > maxSafeUdpRelayPacketSize && maxPacketSize <= maxLegacyUdpRelayPacketSize {
		maxPacketSize = maxSafeUdpRelayPacketSize
	}

	clients := make([]TuicClientSettings, 0, len(parsed.Clients))
	for _, c := range parsed.Clients {
		if c.Enable != nil && !*c.Enable {
			continue
		}
		uuidVal := strings.ToLower(strings.TrimSpace(c.UUID))
		if uuidVal == "" {
			uuidVal = strings.ToLower(strings.TrimSpace(c.ID))
		}
		if uuidVal == "" || c.Password == "" {
			continue
		}
		clients = append(clients, TuicClientSettings{
			TrafficID: c.TrafficID,
			UUID:      uuidVal,
			Password:  c.Password,
			Email:     c.Email,
		})
	}

	return Instance{
		Id:                    ib.Id,
		Tag:                   ib.Tag,
		Listen:                ib.Listen,
		Port:                  ib.Port,
		Certificate:           cert,
		PrivateKey:            key,
		CongestionControl:     cc,
		ALPN:                  alpn,
		UDPRelayMode:          udpRelayMode,
		ZeroRTTHandshake:      zeroRtt,
		LogLevel:              logLevel,
		MaxIdleTime:           maxIdle,
		AuthenticationTimeout: authTimeout,
		MaxUdpRelayPacketSize: maxPacketSize,
		SNI:                   sni,
		Clients:               clients,
	}, true
}

func NormalizeCongestionControl(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "bbr":
		return "bbr", nil
	case "cubic":
		return "cubic", nil
	case "new_reno", "reno":
		return "new_reno", nil
	default:
		return "", fmt.Errorf("tuic: unsupported congestion controller %q", value)
	}
}
