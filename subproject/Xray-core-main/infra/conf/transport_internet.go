package conf

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/platform/filesystem"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/finalmask/fragment"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/custom"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/dns"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/dtls"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/srtp"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/utp"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/wechat"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/wireguard"
	"github.com/xtls/xray-core/transport/internet/finalmask/mkcp/aes128gcm"
	"github.com/xtls/xray-core/transport/internet/finalmask/mkcp/original"
	"github.com/xtls/xray-core/transport/internet/finalmask/noise"
	"github.com/xtls/xray-core/transport/internet/finalmask/salamander"
	finalsudoku "github.com/xtls/xray-core/transport/internet/finalmask/sudoku"
	"github.com/xtls/xray-core/transport/internet/finalmask/xdns"
	"github.com/xtls/xray-core/transport/internet/finalmask/xicmp"
	"github.com/xtls/xray-core/transport/internet/httpupgrade"
	"github.com/xtls/xray-core/transport/internet/hysteria"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion/bbr"
	"github.com/xtls/xray-core/transport/internet/kcp"
	"github.com/xtls/xray-core/transport/internet/reality"
	"github.com/xtls/xray-core/transport/internet/splithttp"
	"github.com/xtls/xray-core/transport/internet/tcp"
	"github.com/xtls/xray-core/transport/internet/tls"
	"github.com/xtls/xray-core/transport/internet/websocket"
	"google.golang.org/protobuf/proto"
)

var (
	tcpHeaderLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"none": func() interface{} { return new(NoOpConnectionAuthenticator) },
		"http": func() interface{} { return new(Authenticator) },
	}, "type", "")
)

type KCPConfig struct {
	Mtu             *uint32         `json:"mtu"`
	Tti             *uint32         `json:"tti"`
	UpCap           *uint32         `json:"uplinkCapacity"`
	DownCap         *uint32         `json:"downlinkCapacity"`
	Congestion      *bool           `json:"congestion"`
	ReadBufferSize  *uint32         `json:"readBufferSize"`
	WriteBufferSize *uint32         `json:"writeBufferSize"`
	HeaderConfig    json.RawMessage `json:"header"`
	Seed            *string         `json:"seed"`
}

// Build implements Buildable.
func (c *KCPConfig) Build() (proto.Message, error) {
	config := new(kcp.Config)

	if c.Mtu != nil {
		mtu := *c.Mtu
		// if mtu < 576 || mtu > 1460 {
		// 	return nil, errors.New("invalid mKCP MTU size: ", mtu).AtError()
		// }
		config.Mtu = &kcp.MTU{Value: mtu}
	}
	if c.Tti != nil {
		tti := *c.Tti
		if tti < 10 || tti > 5000 {
			return nil, errors.New("invalid mKCP TTI: ", tti).AtError()
		}
		config.Tti = &kcp.TTI{Value: tti}
	}
	if c.UpCap != nil {
		config.UplinkCapacity = &kcp.UplinkCapacity{Value: *c.UpCap}
	}
	if c.DownCap != nil {
		config.DownlinkCapacity = &kcp.DownlinkCapacity{Value: *c.DownCap}
	}
	if c.Congestion != nil {
		config.Congestion = *c.Congestion
	}
	if c.ReadBufferSize != nil {
		size := *c.ReadBufferSize
		if size > 0 {
			config.ReadBuffer = &kcp.ReadBuffer{Size: size * 1024 * 1024}
		} else {
			config.ReadBuffer = &kcp.ReadBuffer{Size: 512 * 1024}
		}
	}
	if c.WriteBufferSize != nil {
		size := *c.WriteBufferSize
		if size > 0 {
			config.WriteBuffer = &kcp.WriteBuffer{Size: size * 1024 * 1024}
		} else {
			config.WriteBuffer = &kcp.WriteBuffer{Size: 512 * 1024}
		}
	}
	if c.HeaderConfig != nil || c.Seed != nil {
		return nil, errors.PrintRemovedFeatureError("mkcp header & seed", "finalmask/udp header-* & mkcp-original & mkcp-aes128gcm")
	}

	return config, nil
}

type TCPConfig struct {
	HeaderConfig        json.RawMessage `json:"header"`
	AcceptProxyProtocol bool            `json:"acceptProxyProtocol"`
}

// Build implements Buildable.
func (c *TCPConfig) Build() (proto.Message, error) {
	config := new(tcp.Config)
	if len(c.HeaderConfig) > 0 {
		headerConfig, _, err := tcpHeaderLoader.Load(c.HeaderConfig)
		if err != nil {
			return nil, errors.New("invalid TCP header config").Base(err).AtError()
		}
		ts, err := headerConfig.(Buildable).Build()
		if err != nil {
			return nil, errors.New("invalid TCP header config").Base(err).AtError()
		}
		config.HeaderSettings = serial.ToTypedMessage(ts)
	}
	if c.AcceptProxyProtocol {
		config.AcceptProxyProtocol = c.AcceptProxyProtocol
	}
	return config, nil
}

type WebSocketConfig struct {
	Host                string            `json:"host"`
	Path                string            `json:"path"`
	Headers             map[string]string `json:"headers"`
	AcceptProxyProtocol bool              `json:"acceptProxyProtocol"`
	HeartbeatPeriod     uint32            `json:"heartbeatPeriod"`
}

// Build implements Buildable.
func (c *WebSocketConfig) Build() (proto.Message, error) {
	path := c.Path
	var ed uint32
	if u, err := url.Parse(path); err == nil {
		if q := u.Query(); q.Get("ed") != "" {
			Ed, _ := strconv.Atoi(q.Get("ed"))
			ed = uint32(Ed)
			q.Del("ed")
			u.RawQuery = q.Encode()
			path = u.String()
		}
	}
	// Priority (client): host > serverName > address
	for k, v := range c.Headers {
		if strings.ToLower(k) == "host" {
			errors.PrintDeprecatedFeatureWarning(`"host" in "headers"`, `independent "host"`)
			if c.Host == "" {
				c.Host = v
			}
			delete(c.Headers, k)
		}
	}
	config := &websocket.Config{
		Path:                path,
		Host:                c.Host,
		Header:              c.Headers,
		AcceptProxyProtocol: c.AcceptProxyProtocol,
		Ed:                  ed,
		HeartbeatPeriod:     c.HeartbeatPeriod,
	}
	return config, nil
}

type HttpUpgradeConfig struct {
	Host                string            `json:"host"`
	Path                string            `json:"path"`
	Headers             map[string]string `json:"headers"`
	AcceptProxyProtocol bool              `json:"acceptProxyProtocol"`
}

// Build implements Buildable.
func (c *HttpUpgradeConfig) Build() (proto.Message, error) {
	path := c.Path
	var ed uint32
	if u, err := url.Parse(path); err == nil {
		if q := u.Query(); q.Get("ed") != "" {
			Ed, _ := strconv.Atoi(q.Get("ed"))
			ed = uint32(Ed)
			q.Del("ed")
			u.RawQuery = q.Encode()
			path = u.String()
		}
	}
	// Priority (client): host > serverName > address
	for k := range c.Headers {
		if strings.ToLower(k) == "host" {
			return nil, errors.New(`"headers" can't contain "host"`)
		}
	}
	config := &httpupgrade.Config{
		Path:                path,
		Host:                c.Host,
		Header:              c.Headers,
		AcceptProxyProtocol: c.AcceptProxyProtocol,
		Ed:                  ed,
	}
	return config, nil
}

type SplitHTTPConfig struct {
	Host                 string            `json:"host"`
	Path                 string            `json:"path"`
	Mode                 string            `json:"mode"`
	Headers              map[string]string `json:"headers"`
	XPaddingBytes        Int32Range        `json:"xPaddingBytes"`
	XPaddingObfsMode     bool              `json:"xPaddingObfsMode"`
	XPaddingKey          string            `json:"xPaddingKey"`
	XPaddingHeader       string            `json:"xPaddingHeader"`
	XPaddingPlacement    string            `json:"xPaddingPlacement"`
	XPaddingMethod       string            `json:"xPaddingMethod"`
	UplinkHTTPMethod     string            `json:"uplinkHTTPMethod"`
	SessionPlacement     string            `json:"sessionPlacement"`
	SessionKey           string            `json:"sessionKey"`
	SeqPlacement         string            `json:"seqPlacement"`
	SeqKey               string            `json:"seqKey"`
	UplinkDataPlacement  string            `json:"uplinkDataPlacement"`
	UplinkDataKey        string            `json:"uplinkDataKey"`
	UplinkChunkSize      Int32Range        `json:"uplinkChunkSize"`
	NoGRPCHeader         bool              `json:"noGRPCHeader"`
	NoSSEHeader          bool              `json:"noSSEHeader"`
	ScMaxEachPostBytes   Int32Range        `json:"scMaxEachPostBytes"`
	ScMinPostsIntervalMs Int32Range        `json:"scMinPostsIntervalMs"`
	ScMaxBufferedPosts   int64             `json:"scMaxBufferedPosts"`
	ScStreamUpServerSecs Int32Range        `json:"scStreamUpServerSecs"`
	ServerMaxHeaderBytes int32             `json:"serverMaxHeaderBytes"`
	Xmux                 XmuxConfig        `json:"xmux"`
	DownloadSettings     *StreamConfig     `json:"downloadSettings"`
	Extra                json.RawMessage   `json:"extra"`
}

type XmuxConfig struct {
	MaxConcurrency   Int32Range `json:"maxConcurrency"`
	MaxConnections   Int32Range `json:"maxConnections"`
	CMaxReuseTimes   Int32Range `json:"cMaxReuseTimes"`
	HMaxRequestTimes Int32Range `json:"hMaxRequestTimes"`
	HMaxReusableSecs Int32Range `json:"hMaxReusableSecs"`
	HKeepAlivePeriod int64      `json:"hKeepAlivePeriod"`
}

func newRangeConfig(input Int32Range) *splithttp.RangeConfig {
	return &splithttp.RangeConfig{
		From: input.From,
		To:   input.To,
	}
}

// Build implements Buildable.
func (c *SplitHTTPConfig) Build() (proto.Message, error) {
	if c.Extra != nil {
		var extra SplitHTTPConfig
		if err := json.Unmarshal(c.Extra, &extra); err != nil {
			return nil, errors.New(`Failed to unmarshal "extra".`).Base(err)
		}
		extra.Host = c.Host
		extra.Path = c.Path
		extra.Mode = c.Mode
		c = &extra
	}

	switch c.Mode {
	case "":
		c.Mode = "auto"
	case "auto", "packet-up", "stream-up", "stream-one":
	default:
		return nil, errors.New("unsupported mode: " + c.Mode)
	}

	// Priority (client): host > serverName > address
	for k := range c.Headers {
		if strings.ToLower(k) == "host" {
			return nil, errors.New(`"headers" can't contain "host"`)
		}
	}

	if c.XPaddingBytes != (Int32Range{}) && (c.XPaddingBytes.From <= 0 || c.XPaddingBytes.To <= 0) {
		return nil, errors.New("xPaddingBytes cannot be disabled")
	}

	if c.XPaddingKey == "" {
		c.XPaddingKey = "x_padding"
	}

	if c.XPaddingHeader == "" {
		c.XPaddingHeader = "X-Padding"
	}

	switch c.XPaddingPlacement {
	case "":
		c.XPaddingPlacement = "queryInHeader"
	case "cookie", "header", "query", "queryInHeader":
	default:
		return nil, errors.New("unsupported padding placement: " + c.XPaddingPlacement)
	}

	switch c.XPaddingMethod {
	case "":
		c.XPaddingMethod = "repeat-x"
	case "repeat-x", "tokenish":
	default:
		return nil, errors.New("unsupported padding method: " + c.XPaddingMethod)
	}

	switch c.UplinkDataPlacement {
	case "":
		c.UplinkDataPlacement = splithttp.PlacementAuto
	case splithttp.PlacementAuto, splithttp.PlacementBody:
	case splithttp.PlacementCookie, splithttp.PlacementHeader:
		if c.Mode != "packet-up" {
			return nil, errors.New("UplinkDataPlacement can be " + c.UplinkDataPlacement + " only in packet-up mode")
		}
	default:
		return nil, errors.New("unsupported uplink data placement: " + c.UplinkDataPlacement)
	}

	if c.UplinkHTTPMethod == "" {
		c.UplinkHTTPMethod = "POST"
	}
	c.UplinkHTTPMethod = strings.ToUpper(c.UplinkHTTPMethod)

	if c.UplinkHTTPMethod == "GET" && c.Mode != "packet-up" {
		return nil, errors.New("uplinkHTTPMethod can be GET only in packet-up mode")
	}

	switch c.SessionPlacement {
	case "":
		c.SessionPlacement = "path"
	case "path", "cookie", "header", "query":
	default:
		return nil, errors.New("unsupported session placement: " + c.SessionPlacement)
	}

	switch c.SeqPlacement {
	case "":
		c.SeqPlacement = "path"
	case "path", "cookie", "header", "query":
	default:
		return nil, errors.New("unsupported seq placement: " + c.SeqPlacement)
	}

	if c.SessionPlacement != "path" && c.SessionKey == "" {
		switch c.SessionPlacement {
		case "cookie", "query":
			c.SessionKey = "x_session"
		case "header":
			c.SessionKey = "X-Session"
		}
	}

	if c.SeqPlacement != "path" && c.SeqKey == "" {
		switch c.SeqPlacement {
		case "cookie", "query":
			c.SeqKey = "x_seq"
		case "header":
			c.SeqKey = "X-Seq"
		}
	}

	if c.UplinkDataPlacement != splithttp.PlacementBody && c.UplinkDataKey == "" {
		switch c.UplinkDataPlacement {
		case splithttp.PlacementCookie:
			c.UplinkDataKey = "x_data"
		case splithttp.PlacementAuto, splithttp.PlacementHeader:
			c.UplinkDataKey = "X-Data"
		}
	}

	if c.ServerMaxHeaderBytes < 0 {
		return nil, errors.New("invalid negative value of maxHeaderBytes")
	}

	if c.Xmux.MaxConnections.To > 0 && c.Xmux.MaxConcurrency.To > 0 {
		return nil, errors.New("maxConnections cannot be specified together with maxConcurrency")
	}
	if c.Xmux == (XmuxConfig{}) {
		c.Xmux.MaxConcurrency.From = 1
		c.Xmux.MaxConcurrency.To = 1
		c.Xmux.HMaxRequestTimes.From = 600
		c.Xmux.HMaxRequestTimes.To = 900
		c.Xmux.HMaxReusableSecs.From = 1800
		c.Xmux.HMaxReusableSecs.To = 3000
	}

	config := &splithttp.Config{
		Host:                 c.Host,
		Path:                 c.Path,
		Mode:                 c.Mode,
		Headers:              c.Headers,
		XPaddingBytes:        newRangeConfig(c.XPaddingBytes),
		XPaddingObfsMode:     c.XPaddingObfsMode,
		XPaddingKey:          c.XPaddingKey,
		XPaddingHeader:       c.XPaddingHeader,
		XPaddingPlacement:    c.XPaddingPlacement,
		XPaddingMethod:       c.XPaddingMethod,
		UplinkHTTPMethod:     c.UplinkHTTPMethod,
		SessionPlacement:     c.SessionPlacement,
		SeqPlacement:         c.SeqPlacement,
		SessionKey:           c.SessionKey,
		SeqKey:               c.SeqKey,
		UplinkDataPlacement:  c.UplinkDataPlacement,
		UplinkDataKey:        c.UplinkDataKey,
		UplinkChunkSize:      newRangeConfig(c.UplinkChunkSize),
		NoGRPCHeader:         c.NoGRPCHeader,
		NoSSEHeader:          c.NoSSEHeader,
		ScMaxEachPostBytes:   newRangeConfig(c.ScMaxEachPostBytes),
		ScMinPostsIntervalMs: newRangeConfig(c.ScMinPostsIntervalMs),
		ScMaxBufferedPosts:   c.ScMaxBufferedPosts,
		ScStreamUpServerSecs: newRangeConfig(c.ScStreamUpServerSecs),
		ServerMaxHeaderBytes: c.ServerMaxHeaderBytes,
		Xmux: &splithttp.XmuxConfig{
			MaxConcurrency:   newRangeConfig(c.Xmux.MaxConcurrency),
			MaxConnections:   newRangeConfig(c.Xmux.MaxConnections),
			CMaxReuseTimes:   newRangeConfig(c.Xmux.CMaxReuseTimes),
			HMaxRequestTimes: newRangeConfig(c.Xmux.HMaxRequestTimes),
			HMaxReusableSecs: newRangeConfig(c.Xmux.HMaxReusableSecs),
			HKeepAlivePeriod: c.Xmux.HKeepAlivePeriod,
		},
	}

	if c.DownloadSettings != nil {
		if c.Mode == "stream-one" {
			return nil, errors.New(`Can not use "downloadSettings" in "stream-one" mode.`)
		}
		var err error
		if config.DownloadSettings, err = c.DownloadSettings.Build(); err != nil {
			return nil, errors.New(`Failed to build "downloadSettings".`).Base(err)
		}
	}

	return config, nil
}

const (
	Byte     = 1
	Kilobyte = 1024 * Byte
	Megabyte = 1024 * Kilobyte
	Gigabyte = 1024 * Megabyte
	Terabyte = 1024 * Gigabyte
)

type Bandwidth string

func (b Bandwidth) Bps() (uint64, error) {
	s := strings.TrimSpace(strings.ToLower(string(b)))
	if s == "" {
		return 0, nil
	}

	idx := len(s)
	for i, c := range s {
		if (c < '0' || c > '9') && c != '.' {
			idx = i
			break
		}
	}

	numStr := s[:idx]
	unit := strings.TrimSpace(s[idx:])

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	mul := uint64(1)
	switch unit {
	case "", "b", "bps":
		mul = Byte
	case "k", "kb", "kbps":
		mul = Kilobyte
	case "m", "mb", "mbps":
		mul = Megabyte
	case "g", "gb", "gbps":
		mul = Gigabyte
	case "t", "tb", "tbps":
		mul = Terabyte
	default:
		return 0, errors.New("unsupported unit: " + unit)
	}

	return uint64(val*float64(mul)) / 8, nil
}

type UdpHop struct {
	PortList json.RawMessage `json:"ports"`
	Interval *Int32Range     `json:"interval"`
}

type Masquerade struct {
	Type string `json:"type"`

	Dir string `json:"dir"`

	Url         string `json:"url"`
	RewriteHost bool   `json:"rewriteHost"`
	Insecure    bool   `json:"insecure"`

	Content    string            `json:"content"`
	Headers    map[string]string `json:"headers"`
	StatusCode int32             `json:"statusCode"`
}

type HysteriaConfig struct {
	Version int32  `json:"version"`
	Auth    string `json:"auth"`

	Congestion *string    `json:"congestion"`
	Up         *Bandwidth `json:"up"`
	Down       *Bandwidth `json:"down"`
	UdpHop     *UdpHop    `json:"udphop"`

	UdpIdleTimeout int64      `json:"udpIdleTimeout"`
	Masquerade     Masquerade `json:"masquerade"`
}

func (c *HysteriaConfig) Build() (proto.Message, error) {
	if c.Version != 2 {
		return nil, errors.New("version != 2")
	}

	if c.Congestion != nil || c.Up != nil || c.Down != nil || c.UdpHop != nil {
		errors.LogWarning(context.Background(), "congestion & up & down & udphop move to finalmask/quicParams")
	}

	if c.UdpIdleTimeout != 0 && (c.UdpIdleTimeout < 2 || c.UdpIdleTimeout > 600) {
		return nil, errors.New("UdpIdleTimeout must be between 2 and 600")
	}

	config := &hysteria.Config{}
	config.Version = c.Version
	config.Auth = c.Auth
	config.UdpIdleTimeout = c.UdpIdleTimeout
	config.MasqType = c.Masquerade.Type
	config.MasqFile = c.Masquerade.Dir
	config.MasqUrl = c.Masquerade.Url
	config.MasqUrlRewriteHost = c.Masquerade.RewriteHost
	config.MasqUrlInsecure = c.Masquerade.Insecure
	config.MasqString = c.Masquerade.Content
	config.MasqStringHeaders = c.Masquerade.Headers
	config.MasqStringStatusCode = c.Masquerade.StatusCode

	if config.UdpIdleTimeout == 0 {
		config.UdpIdleTimeout = 60
	}

	return config, nil
}

func readFileOrString(f string, s []string) ([]byte, error) {
	if len(f) > 0 {
		return filesystem.ReadCert(f)
	}
	if len(s) > 0 {
		return []byte(strings.Join(s, "\n")), nil
	}
	return nil, errors.New("both file and bytes are empty.")
}

type TLSCertConfig struct {
	CertFile       string   `json:"certificateFile"`
	CertStr        []string `json:"certificate"`
	KeyFile        string   `json:"keyFile"`
	KeyStr         []string `json:"key"`
	Usage          string   `json:"usage"`
	OcspStapling   uint64   `json:"ocspStapling"`
	OneTimeLoading bool     `json:"oneTimeLoading"`
	BuildChain     bool     `json:"buildChain"`
}

// Build implements Buildable.
func (c *TLSCertConfig) Build() (*tls.Certificate, error) {
	certificate := new(tls.Certificate)

	cert, err := readFileOrString(c.CertFile, c.CertStr)
	if err != nil {
		return nil, errors.New("failed to parse certificate").Base(err)
	}
	certificate.Certificate = cert
	certificate.CertificatePath = c.CertFile

	if len(c.KeyFile) > 0 || len(c.KeyStr) > 0 {
		key, err := readFileOrString(c.KeyFile, c.KeyStr)
		if err != nil {
			return nil, errors.New("failed to parse key").Base(err)
		}
		certificate.Key = key
		certificate.KeyPath = c.KeyFile
	}

	switch strings.ToLower(c.Usage) {
	case "encipherment":
		certificate.Usage = tls.Certificate_ENCIPHERMENT
	case "verify":
		certificate.Usage = tls.Certificate_AUTHORITY_VERIFY
	case "issue":
		certificate.Usage = tls.Certificate_AUTHORITY_ISSUE
	default:
		certificate.Usage = tls.Certificate_ENCIPHERMENT
	}
	if certificate.KeyPath == "" && certificate.CertificatePath == "" {
		certificate.OneTimeLoading = true
	} else {
		certificate.OneTimeLoading = c.OneTimeLoading
	}
	certificate.OcspStapling = c.OcspStapling
	certificate.BuildChain = c.BuildChain

	return certificate, nil
}

type QuicParamsConfig struct {
	Congestion                  string    `json:"congestion"`
	Debug                       bool      `json:"debug"`
	BbrProfile                  string    `json:"bbrProfile"`
	BrutalUp                    Bandwidth `json:"brutalUp"`
	BrutalDown                  Bandwidth `json:"brutalDown"`
	UdpHop                      UdpHop    `json:"udpHop"`
	InitStreamReceiveWindow     uint64    `json:"initStreamReceiveWindow"`
	MaxStreamReceiveWindow      uint64    `json:"maxStreamReceiveWindow"`
	InitConnectionReceiveWindow uint64    `json:"initConnectionReceiveWindow"`
	MaxConnectionReceiveWindow  uint64    `json:"maxConnectionReceiveWindow"`
	MaxIdleTimeout              int64     `json:"maxIdleTimeout"`
	KeepAlivePeriod             int64     `json:"keepAlivePeriod"`
	DisablePathMTUDiscovery     bool      `json:"disablePathMTUDiscovery"`
	MaxIncomingStreams          int64     `json:"maxIncomingStreams"`
}

type TLSConfig struct {
	AllowInsecure           bool             `json:"allowInsecure"`
	Certs                   []*TLSCertConfig `json:"certificates"`
	ServerName              string           `json:"serverName"`
	ALPN                    *StringList      `json:"alpn"`
	EnableSessionResumption bool             `json:"enableSessionResumption"`
	DisableSystemRoot       bool             `json:"disableSystemRoot"`
	MinVersion              string           `json:"minVersion"`
	MaxVersion              string           `json:"maxVersion"`
	CipherSuites            string           `json:"cipherSuites"`
	Fingerprint             string           `json:"fingerprint"`
	RejectUnknownSNI        bool             `json:"rejectUnknownSni"`
	CurvePreferences        *StringList      `json:"curvePreferences"`
	MasterKeyLog            string           `json:"masterKeyLog"`
	PinnedPeerCertSha256    string           `json:"pinnedPeerCertSha256"`
	VerifyPeerCertByName    string           `json:"verifyPeerCertByName"`
	VerifyPeerCertInNames   []string         `json:"verifyPeerCertInNames"`
	ECHServerKeys           string           `json:"echServerKeys"`
	ECHConfigList           string           `json:"echConfigList"`
	ECHForceQuery           string           `json:"echForceQuery"`
	ECHSocketSettings       *SocketConfig    `json:"echSockopt"`
}

// Build implements Buildable.
func (c *TLSConfig) Build() (proto.Message, error) {
	config := new(tls.Config)
	config.Certificate = make([]*tls.Certificate, len(c.Certs))
	for idx, certConf := range c.Certs {
		cert, err := certConf.Build()
		if err != nil {
			return nil, err
		}
		config.Certificate[idx] = cert
	}
	serverName := c.ServerName
	if len(c.ServerName) > 0 {
		config.ServerName = serverName
	}
	if c.ALPN != nil && len(*c.ALPN) > 0 {
		config.NextProtocol = []string(*c.ALPN)
	}
	if len(config.NextProtocol) > 1 {
		for _, p := range config.NextProtocol {
			if tls.IsFromMitm(p) {
				return nil, errors.New(`only one element is allowed in "alpn" when using "fromMitm" in it`)
			}
		}
	}
	if c.CurvePreferences != nil && len(*c.CurvePreferences) > 0 {
		config.CurvePreferences = []string(*c.CurvePreferences)
	}
	config.EnableSessionResumption = c.EnableSessionResumption
	config.DisableSystemRoot = c.DisableSystemRoot
	config.MinVersion = c.MinVersion
	config.MaxVersion = c.MaxVersion
	config.CipherSuites = c.CipherSuites
	config.Fingerprint = strings.ToLower(c.Fingerprint)
	if config.Fingerprint != "unsafe" && tls.GetFingerprint(config.Fingerprint) == nil {
		return nil, errors.New(`unknown "fingerprint": `, config.Fingerprint)
	}
	config.RejectUnknownSni = c.RejectUnknownSNI
	config.MasterKeyLog = c.MasterKeyLog

	if c.AllowInsecure {
		if time.Now().After(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
			return nil, errors.PrintRemovedFeatureError(`"allowInsecure"`, `"pinnedPeerCertSha256"`)
		} else {
			errors.LogWarning(context.Background(), `"allowInsecure" will be removed automatically after 2026-06-01, please use "pinnedPeerCertSha256"(pcs) and "verifyPeerCertByName"(vcn) instead, PLEASE CONTACT YOUR SERVICE PROVIDER (AIRPORT)`)
			config.AllowInsecure = true
		}
	}
	if c.PinnedPeerCertSha256 != "" {
		for v := range strings.SplitSeq(c.PinnedPeerCertSha256, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			// remove colons for OpenSSL format
			hashValue, err := hex.DecodeString(strings.ReplaceAll(v, ":", ""))
			if err != nil {
				return nil, err
			}
			if len(hashValue) != 32 {
				return nil, errors.New("incorrect pinnedPeerCertSha256 length: ", v)
			}
			config.PinnedPeerCertSha256 = append(config.PinnedPeerCertSha256, hashValue)
		}
	}

	if c.VerifyPeerCertInNames != nil {
		return nil, errors.PrintRemovedFeatureError(`"verifyPeerCertInNames"`, `"verifyPeerCertByName"`)
	}
	if c.VerifyPeerCertByName != "" {
		for v := range strings.SplitSeq(c.VerifyPeerCertByName, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			config.VerifyPeerCertByName = append(config.VerifyPeerCertByName, v)
		}
	}

	if c.ECHServerKeys != "" {
		EchPrivateKey, err := base64.StdEncoding.DecodeString(c.ECHServerKeys)
		if err != nil {
			return nil, errors.New("invalid ECH Config", c.ECHServerKeys)
		}
		config.EchServerKeys = EchPrivateKey
	}
	switch c.ECHForceQuery {
	case "none", "half", "full", "":
		config.EchForceQuery = c.ECHForceQuery
	default:
		return nil, errors.New(`invalid "echForceQuery": `, c.ECHForceQuery)
	}
	config.EchForceQuery = c.ECHForceQuery
	config.EchConfigList = c.ECHConfigList
	if c.ECHSocketSettings != nil {
		ss, err := c.ECHSocketSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build ech sockopt.").Base(err)
		}
		config.EchSocketSettings = ss
	}

	return config, nil
}

type LimitFallback struct {
	AfterBytes       uint64
	BytesPerSec      uint64
	BurstBytesPerSec uint64
}

type REALITYConfig struct {
	MasterKeyLog string          `json:"masterKeyLog"`
	Show         bool            `json:"show"`
	Target       json.RawMessage `json:"target"`
	Dest         json.RawMessage `json:"dest"`
	Type         string          `json:"type"`
	Xver         uint64          `json:"xver"`
	ServerNames  []string        `json:"serverNames"`
	PrivateKey   string          `json:"privateKey"`
	MinClientVer string          `json:"minClientVer"`
	MaxClientVer string          `json:"maxClientVer"`
	MaxTimeDiff  uint64          `json:"maxTimeDiff"`
	ShortIds     []string        `json:"shortIds"`
	Mldsa65Seed  string          `json:"mldsa65Seed"`

	LimitFallbackUpload   LimitFallback `json:"limitFallbackUpload"`
	LimitFallbackDownload LimitFallback `json:"limitFallbackDownload"`

	Fingerprint   string `json:"fingerprint"`
	ServerName    string `json:"serverName"`
	Password      string `json:"password"`
	PublicKey     string `json:"publicKey"`
	ShortId       string `json:"shortId"`
	Mldsa65Verify string `json:"mldsa65Verify"`
	SpiderX       string `json:"spiderX"`
}

func (c *REALITYConfig) Build() (proto.Message, error) {
	config := new(reality.Config)
	config.MasterKeyLog = c.MasterKeyLog
	config.Show = c.Show
	var err error
	if c.Target != nil {
		c.Dest = c.Target
	}
	if c.Dest != nil {
		var i uint16
		var s string
		if err = json.Unmarshal(c.Dest, &i); err == nil {
			s = strconv.Itoa(int(i))
		} else {
			_ = json.Unmarshal(c.Dest, &s)
		}
		if c.Type == "" && s != "" {
			switch s[0] {
			case '@', '/':
				c.Type = "unix"
				if s[0] == '@' && len(s) > 1 && s[1] == '@' && (runtime.GOOS == "linux" || runtime.GOOS == "android") {
					fullAddr := make([]byte, len(syscall.RawSockaddrUnix{}.Path)) // may need padding to work with haproxy
					copy(fullAddr, s[1:])
					s = string(fullAddr)
				}
			default:
				if _, err = strconv.Atoi(s); err == nil {
					s = "localhost:" + s
				}
				if _, _, err = net.SplitHostPort(s); err == nil {
					c.Type = "tcp"
				}
			}
		}
		if c.Type == "" {
			return nil, errors.New(`please fill in a valid value for "target"`)
		}
		if c.Xver > 2 {
			return nil, errors.New(`invalid PROXY protocol version, "xver" only accepts 0, 1, 2`)
		}
		if len(c.ServerNames) == 0 {
			return nil, errors.New(`empty "serverNames"`)
		}
		if c.PrivateKey == "" {
			return nil, errors.New(`empty "privateKey"`)
		}
		if config.PrivateKey, err = base64.RawURLEncoding.DecodeString(c.PrivateKey); err != nil || len(config.PrivateKey) != 32 {
			return nil, errors.New(`invalid "privateKey": `, c.PrivateKey)
		}
		if c.MinClientVer != "" {
			config.MinClientVer = make([]byte, 3)
			var u uint64
			for i, s := range strings.Split(c.MinClientVer, ".") {
				if i == 3 {
					return nil, errors.New(`invalid "minClientVer": `, c.MinClientVer)
				}
				if u, err = strconv.ParseUint(s, 10, 8); err != nil {
					return nil, errors.New(`"minClientVer[`, i, `]" should be less than 256`)
				} else {
					config.MinClientVer[i] = byte(u)
				}
			}
		}
		if c.MaxClientVer != "" {
			config.MaxClientVer = make([]byte, 3)
			var u uint64
			for i, s := range strings.Split(c.MaxClientVer, ".") {
				if i == 3 {
					return nil, errors.New(`invalid "maxClientVer": `, c.MaxClientVer)
				}
				if u, err = strconv.ParseUint(s, 10, 8); err != nil {
					return nil, errors.New(`"maxClientVer[`, i, `]" should be less than 256`)
				} else {
					config.MaxClientVer[i] = byte(u)
				}
			}
		}
		if len(c.ShortIds) == 0 {
			return nil, errors.New(`empty "shortIds"`)
		}
		config.ShortIds = make([][]byte, len(c.ShortIds))
		for i, s := range c.ShortIds {
			if len(s) > 16 {
				return nil, errors.New(`too long "shortIds[`, i, `]": `, s)
			}
			config.ShortIds[i] = make([]byte, 8)
			if _, err = hex.Decode(config.ShortIds[i], []byte(s)); err != nil {
				return nil, errors.New(`invalid "shortIds[`, i, `]": `, s)
			}
		}
		config.Dest = s
		config.Type = c.Type
		config.Xver = c.Xver
		config.ServerNames = c.ServerNames
		config.MaxTimeDiff = c.MaxTimeDiff

		if c.Mldsa65Seed != "" {
			if c.Mldsa65Seed == c.PrivateKey {
				return nil, errors.New(`"mldsa65Seed" and "privateKey" can not be the same value: `, c.Mldsa65Seed)
			}
			if config.Mldsa65Seed, err = base64.RawURLEncoding.DecodeString(c.Mldsa65Seed); err != nil || len(config.Mldsa65Seed) != 32 {
				return nil, errors.New(`invalid "mldsa65Seed": `, c.Mldsa65Seed)
			}
		}

		for _, sn := range config.ServerNames {
			if strings.Contains(sn, "apple") || strings.Contains(sn, "icloud") {
				errors.LogWarning(context.Background(), `REALITY: Choosing apple, icloud, etc. as the target may get your IP blocked by the GFW`)
			}
		}

		config.LimitFallbackUpload = new(reality.LimitFallback)
		config.LimitFallbackUpload.AfterBytes = c.LimitFallbackUpload.AfterBytes
		config.LimitFallbackUpload.BytesPerSec = c.LimitFallbackUpload.BytesPerSec
		config.LimitFallbackUpload.BurstBytesPerSec = c.LimitFallbackUpload.BurstBytesPerSec
		config.LimitFallbackDownload = new(reality.LimitFallback)
		config.LimitFallbackDownload.AfterBytes = c.LimitFallbackDownload.AfterBytes
		config.LimitFallbackDownload.BytesPerSec = c.LimitFallbackDownload.BytesPerSec
		config.LimitFallbackDownload.BurstBytesPerSec = c.LimitFallbackDownload.BurstBytesPerSec
	} else {
		config.Fingerprint = strings.ToLower(c.Fingerprint)
		if config.Fingerprint == "unsafe" || config.Fingerprint == "hellogolang" {
			return nil, errors.New(`invalid "fingerprint": `, config.Fingerprint)
		}
		if tls.GetFingerprint(config.Fingerprint) == nil {
			return nil, errors.New(`unknown "fingerprint": `, config.Fingerprint)
		}
		if len(c.ServerNames) != 0 {
			return nil, errors.New(`non-empty "serverNames", please use "serverName" instead`)
		}
		if c.Password != "" {
			c.PublicKey = c.Password
		}
		if c.PublicKey == "" {
			return nil, errors.New(`empty "password"`)
		}
		if config.PublicKey, err = base64.RawURLEncoding.DecodeString(c.PublicKey); err != nil || len(config.PublicKey) != 32 {
			return nil, errors.New(`invalid "password": `, c.PublicKey)
		}
		if len(c.ShortIds) != 0 {
			return nil, errors.New(`non-empty "shortIds", please use "shortId" instead`)
		}
		if len(c.ShortId) > 16 {
			return nil, errors.New(`too long "shortId": `, c.ShortId)
		}
		config.ShortId = make([]byte, 8)
		if _, err = hex.Decode(config.ShortId, []byte(c.ShortId)); err != nil {
			return nil, errors.New(`invalid "shortId": `, c.ShortId)
		}
		if c.Mldsa65Verify != "" {
			if config.Mldsa65Verify, err = base64.RawURLEncoding.DecodeString(c.Mldsa65Verify); err != nil || len(config.Mldsa65Verify) != 1952 {
				return nil, errors.New(`invalid "mldsa65Verify": `, c.Mldsa65Verify)
			}
		}
		if c.SpiderX == "" {
			c.SpiderX = "/"
		}
		if c.SpiderX[0] != '/' {
			return nil, errors.New(`invalid "spiderX": `, c.SpiderX)
		}
		config.SpiderY = make([]int64, 10)
		u, _ := url.Parse(c.SpiderX)
		q := u.Query()
		parse := func(param string, index int) {
			if q.Get(param) != "" {
				s := strings.Split(q.Get(param), "-")
				if len(s) == 1 {
					config.SpiderY[index], _ = strconv.ParseInt(s[0], 10, 64)
					config.SpiderY[index+1], _ = strconv.ParseInt(s[0], 10, 64)
				} else {
					config.SpiderY[index], _ = strconv.ParseInt(s[0], 10, 64)
					config.SpiderY[index+1], _ = strconv.ParseInt(s[1], 10, 64)
				}
			}
			q.Del(param)
		}
		parse("p", 0) // padding
		parse("c", 2) // concurrency
		parse("t", 4) // times
		parse("i", 6) // interval
		parse("r", 8) // return
		u.RawQuery = q.Encode()
		config.SpiderX = u.String()
		config.ServerName = c.ServerName
	}
	return config, nil
}

type TransportProtocol string

// Build implements Buildable.
func (p TransportProtocol) Build() (string, error) {
	switch strings.ToLower(string(p)) {
	case "raw", "tcp":
		return "tcp", nil
	case "xhttp", "splithttp":
		return "splithttp", nil
	case "kcp", "mkcp":
		return "mkcp", nil
	case "grpc":
		errors.PrintNonRemovalDeprecatedFeatureWarning("gRPC transport (with unnecessary costs, etc.)", "XHTTP stream-up H2")
		return "grpc", nil
	case "ws", "websocket":
		errors.PrintNonRemovalDeprecatedFeatureWarning("WebSocket transport (with ALPN http/1.1, etc.)", "XHTTP H2 & H3")
		return "websocket", nil
	case "httpupgrade":
		errors.PrintNonRemovalDeprecatedFeatureWarning("HTTPUpgrade transport (with ALPN http/1.1, etc.)", "XHTTP H2 & H3")
		return "httpupgrade", nil
	case "h2", "h3", "http":
		return "", errors.PrintRemovedFeatureError("HTTP transport (without header padding, etc.)", "XHTTP stream-one H2 & H3")
	case "quic":
		return "", errors.PrintRemovedFeatureError("QUIC transport (without web service, etc.)", "XHTTP stream-one H3")
	case "hysteria":
		return "hysteria", nil
	default:
		return "", errors.New("Config: unknown transport protocol: ", p)
	}
}

type CustomSockoptConfig struct {
	Syetem  string `json:"system"`
	Network string `json:"network"`
	Level   string `json:"level"`
	Opt     string `json:"opt"`
	Value   string `json:"value"`
	Type    string `json:"type"`
}

type HappyEyeballsConfig struct {
	PrioritizeIPv6   bool   `json:"prioritizeIPv6"`
	TryDelayMs       uint64 `json:"tryDelayMs"`
	Interleave       uint32 `json:"interleave"`
	MaxConcurrentTry uint32 `json:"maxConcurrentTry"`
}

func (h *HappyEyeballsConfig) UnmarshalJSON(data []byte) error {
	var innerHappyEyeballsConfig = struct {
		PrioritizeIPv6   bool   `json:"prioritizeIPv6"`
		TryDelayMs       uint64 `json:"tryDelayMs"`
		Interleave       uint32 `json:"interleave"`
		MaxConcurrentTry uint32 `json:"maxConcurrentTry"`
	}{PrioritizeIPv6: false, Interleave: 1, TryDelayMs: 0, MaxConcurrentTry: 4}
	if err := json.Unmarshal(data, &innerHappyEyeballsConfig); err != nil {
		return err
	}
	h.PrioritizeIPv6 = innerHappyEyeballsConfig.PrioritizeIPv6
	h.TryDelayMs = innerHappyEyeballsConfig.TryDelayMs
	h.Interleave = innerHappyEyeballsConfig.Interleave
	h.MaxConcurrentTry = innerHappyEyeballsConfig.MaxConcurrentTry
	return nil
}

type SocketConfig struct {
	Mark                  int32                  `json:"mark"`
	TFO                   interface{}            `json:"tcpFastOpen"`
	TProxy                string                 `json:"tproxy"`
	AcceptProxyProtocol   bool                   `json:"acceptProxyProtocol"`
	DomainStrategy        string                 `json:"domainStrategy"`
	DialerProxy           string                 `json:"dialerProxy"`
	TCPKeepAliveInterval  int32                  `json:"tcpKeepAliveInterval"`
	TCPKeepAliveIdle      int32                  `json:"tcpKeepAliveIdle"`
	TCPCongestion         string                 `json:"tcpCongestion"`
	TCPWindowClamp        int32                  `json:"tcpWindowClamp"`
	TCPMaxSeg             int32                  `json:"tcpMaxSeg"`
	Penetrate             bool                   `json:"penetrate"`
	TCPUserTimeout        int32                  `json:"tcpUserTimeout"`
	V6only                bool                   `json:"v6only"`
	Interface             string                 `json:"interface"`
	TcpMptcp              bool                   `json:"tcpMptcp"`
	CustomSockopt         []*CustomSockoptConfig `json:"customSockopt"`
	AddressPortStrategy   string                 `json:"addressPortStrategy"`
	HappyEyeballsSettings *HappyEyeballsConfig   `json:"happyEyeballs"`
	TrustedXForwardedFor  []string               `json:"trustedXForwardedFor"`
}

// Build implements Buildable.
func (c *SocketConfig) Build() (*internet.SocketConfig, error) {
	tfo := int32(0) // don't invoke setsockopt() for TFO
	if c.TFO != nil {
		switch v := c.TFO.(type) {
		case bool:
			if v {
				tfo = 256
			} else {
				tfo = -1 // TFO need to be disabled
			}
		case float64:
			tfo = int32(math.Min(v, math.MaxInt32))
		default:
			return nil, errors.New("tcpFastOpen: only boolean and integer value is acceptable")
		}
	}
	var tproxy internet.SocketConfig_TProxyMode
	switch strings.ToLower(c.TProxy) {
	case "tproxy":
		tproxy = internet.SocketConfig_TProxy
	case "redirect":
		tproxy = internet.SocketConfig_Redirect
	default:
		tproxy = internet.SocketConfig_Off
	}

	dStrategy := internet.DomainStrategy_AS_IS
	switch strings.ToLower(c.DomainStrategy) {
	case "asis", "":
		dStrategy = internet.DomainStrategy_AS_IS
	case "useip":
		dStrategy = internet.DomainStrategy_USE_IP
	case "useipv4":
		dStrategy = internet.DomainStrategy_USE_IP4
	case "useipv6":
		dStrategy = internet.DomainStrategy_USE_IP6
	case "useipv4v6":
		dStrategy = internet.DomainStrategy_USE_IP46
	case "useipv6v4":
		dStrategy = internet.DomainStrategy_USE_IP64
	case "forceip":
		dStrategy = internet.DomainStrategy_FORCE_IP
	case "forceipv4":
		dStrategy = internet.DomainStrategy_FORCE_IP4
	case "forceipv6":
		dStrategy = internet.DomainStrategy_FORCE_IP6
	case "forceipv4v6":
		dStrategy = internet.DomainStrategy_FORCE_IP46
	case "forceipv6v4":
		dStrategy = internet.DomainStrategy_FORCE_IP64
	default:
		return nil, errors.New("unsupported domain strategy: ", c.DomainStrategy)
	}

	var customSockopts []*internet.CustomSockopt

	for _, copt := range c.CustomSockopt {
		customSockopt := &internet.CustomSockopt{
			System:  copt.Syetem,
			Network: copt.Network,
			Level:   copt.Level,
			Opt:     copt.Opt,
			Value:   copt.Value,
			Type:    copt.Type,
		}
		customSockopts = append(customSockopts, customSockopt)
	}

	addressPortStrategy := internet.AddressPortStrategy_None
	switch strings.ToLower(c.AddressPortStrategy) {
	case "none", "":
		addressPortStrategy = internet.AddressPortStrategy_None
	case "srvportonly":
		addressPortStrategy = internet.AddressPortStrategy_SrvPortOnly
	case "srvaddressonly":
		addressPortStrategy = internet.AddressPortStrategy_SrvAddressOnly
	case "srvportandaddress":
		addressPortStrategy = internet.AddressPortStrategy_SrvPortAndAddress
	case "txtportonly":
		addressPortStrategy = internet.AddressPortStrategy_TxtPortOnly
	case "txtaddressonly":
		addressPortStrategy = internet.AddressPortStrategy_TxtAddressOnly
	case "txtportandaddress":
		addressPortStrategy = internet.AddressPortStrategy_TxtPortAndAddress
	default:
		return nil, errors.New("unsupported address and port strategy: ", c.AddressPortStrategy)
	}

	var happyEyeballs = &internet.HappyEyeballsConfig{Interleave: 1, PrioritizeIpv6: false, TryDelayMs: 0, MaxConcurrentTry: 4}
	if c.HappyEyeballsSettings != nil {
		happyEyeballs.PrioritizeIpv6 = c.HappyEyeballsSettings.PrioritizeIPv6
		happyEyeballs.Interleave = c.HappyEyeballsSettings.Interleave
		happyEyeballs.TryDelayMs = c.HappyEyeballsSettings.TryDelayMs
		happyEyeballs.MaxConcurrentTry = c.HappyEyeballsSettings.MaxConcurrentTry
	}

	return &internet.SocketConfig{
		Mark:                 c.Mark,
		Tfo:                  tfo,
		Tproxy:               tproxy,
		DomainStrategy:       dStrategy,
		AcceptProxyProtocol:  c.AcceptProxyProtocol,
		DialerProxy:          c.DialerProxy,
		TcpKeepAliveInterval: c.TCPKeepAliveInterval,
		TcpKeepAliveIdle:     c.TCPKeepAliveIdle,
		TcpCongestion:        c.TCPCongestion,
		TcpWindowClamp:       c.TCPWindowClamp,
		TcpMaxSeg:            c.TCPMaxSeg,
		Penetrate:            c.Penetrate,
		TcpUserTimeout:       c.TCPUserTimeout,
		V6Only:               c.V6only,
		Interface:            c.Interface,
		TcpMptcp:             c.TcpMptcp,
		CustomSockopt:        customSockopts,
		AddressPortStrategy:  addressPortStrategy,
		HappyEyeballs:        happyEyeballs,
		TrustedXForwardedFor: c.TrustedXForwardedFor,
	}, nil
}

func PraseByteSlice(data json.RawMessage, typ string) ([]byte, error) {
	switch strings.ToLower(typ) {
	case "", "array":
		if len(data) == 0 {
			return data, nil
		}
		var packet []byte
		if err := json.Unmarshal(data, &packet); err != nil {
			return nil, err
		}
		return packet, nil
	case "str":
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return nil, err
		}
		return []byte(str), nil
	case "hex":
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return nil, err
		}
		return hex.DecodeString(str)
	case "base64":
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return nil, err
		}
		return base64.StdEncoding.DecodeString(str)
	default:
		return nil, errors.New("unknown type")
	}
}

var (
	tcpmaskLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"header-custom": func() interface{} { return new(HeaderCustomTCP) },
		"fragment":      func() interface{} { return new(FragmentMask) },
		"sudoku":        func() interface{} { return new(Sudoku) },
	}, "type", "settings")

	udpmaskLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"header-custom":    func() interface{} { return new(HeaderCustomUDP) },
		"header-dns":       func() interface{} { return new(Dns) },
		"header-dtls":      func() interface{} { return new(Dtls) },
		"header-srtp":      func() interface{} { return new(Srtp) },
		"header-utp":       func() interface{} { return new(Utp) },
		"header-wechat":    func() interface{} { return new(Wechat) },
		"header-wireguard": func() interface{} { return new(Wireguard) },
		"mkcp-original":    func() interface{} { return new(Original) },
		"mkcp-aes128gcm":   func() interface{} { return new(Aes128Gcm) },
		"noise":            func() interface{} { return new(NoiseMask) },
		"salamander":       func() interface{} { return new(Salamander) },
		"sudoku":           func() interface{} { return new(Sudoku) },
		"xdns":             func() interface{} { return new(Xdns) },
		"xicmp":            func() interface{} { return new(Xicmp) },
	}, "type", "settings")
)

type TCPItem struct {
	Delay     Int32Range      `json:"delay"`
	Rand      int32           `json:"rand"`
	RandRange *Int32Range     `json:"randRange"`
	Type      string          `json:"type"`
	Packet    json.RawMessage `json:"packet"`
}

type HeaderCustomTCP struct {
	Clients [][]TCPItem `json:"clients"`
	Servers [][]TCPItem `json:"servers"`
	Errors  [][]TCPItem `json:"errors"`
}

func (c *HeaderCustomTCP) Build() (proto.Message, error) {
	for _, value := range c.Clients {
		for _, item := range value {
			if len(item.Packet) > 0 && item.Rand > 0 {
				return nil, errors.New("len(item.Packet) > 0 && item.Rand > 0")
			}
		}
	}
	for _, value := range c.Servers {
		for _, item := range value {
			if len(item.Packet) > 0 && item.Rand > 0 {
				return nil, errors.New("len(item.Packet) > 0 && item.Rand > 0")
			}
		}
	}
	for _, value := range c.Errors {
		for _, item := range value {
			if len(item.Packet) > 0 && item.Rand > 0 {
				return nil, errors.New("len(item.Packet) > 0 && item.Rand > 0")
			}
		}
	}

	errInvalidRange := errors.New("invalid randRange")

	clients := make([]*custom.TCPSequence, len(c.Clients))
	for i, value := range c.Clients {
		clients[i] = &custom.TCPSequence{}
		for _, item := range value {
			if item.RandRange == nil {
				item.RandRange = &Int32Range{From: 0, To: 255}
			}
			if item.RandRange.From < 0 || item.RandRange.To > 255 {
				return nil, errInvalidRange
			}
			var err error
			if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
				return nil, err
			}
			clients[i].Sequence = append(clients[i].Sequence, &custom.TCPItem{
				DelayMin: int64(item.Delay.From),
				DelayMax: int64(item.Delay.To),
				Rand:     item.Rand,
				RandMin:  item.RandRange.From,
				RandMax:  item.RandRange.To,
				Packet:   item.Packet,
			})
		}
	}

	servers := make([]*custom.TCPSequence, len(c.Servers))
	for i, value := range c.Servers {
		servers[i] = &custom.TCPSequence{}
		for _, item := range value {
			if item.RandRange == nil {
				item.RandRange = &Int32Range{From: 0, To: 255}
			}
			if item.RandRange.From < 0 || item.RandRange.To > 255 {
				return nil, errInvalidRange
			}
			var err error
			if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
				return nil, err
			}
			servers[i].Sequence = append(servers[i].Sequence, &custom.TCPItem{
				DelayMin: int64(item.Delay.From),
				DelayMax: int64(item.Delay.To),
				Rand:     item.Rand,
				RandMin:  item.RandRange.From,
				RandMax:  item.RandRange.To,
				Packet:   item.Packet,
			})
		}
	}

	errors := make([]*custom.TCPSequence, len(c.Errors))
	for i, value := range c.Errors {
		errors[i] = &custom.TCPSequence{}
		for _, item := range value {
			if item.RandRange == nil {
				item.RandRange = &Int32Range{From: 0, To: 255}
			}
			if item.RandRange.From < 0 || item.RandRange.To > 255 {
				return nil, errInvalidRange
			}
			var err error
			if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
				return nil, err
			}
			errors[i].Sequence = append(errors[i].Sequence, &custom.TCPItem{
				DelayMin: int64(item.Delay.From),
				DelayMax: int64(item.Delay.To),
				Rand:     item.Rand,
				RandMin:  item.RandRange.From,
				RandMax:  item.RandRange.To,
				Packet:   item.Packet,
			})
		}
	}

	return &custom.TCPConfig{
		Clients: clients,
		Servers: servers,
		Errors:  errors,
	}, nil
}

type FragmentMask struct {
	Packets  string     `json:"packets"`
	Length   Int32Range `json:"length"`
	Delay    Int32Range `json:"delay"`
	MaxSplit Int32Range `json:"maxSplit"`
}

func (c *FragmentMask) Build() (proto.Message, error) {
	config := &fragment.Config{}

	switch strings.ToLower(c.Packets) {
	case "tlshello":
		config.PacketsFrom = 0
		config.PacketsTo = 1
	case "":
		config.PacketsFrom = 0
		config.PacketsTo = 0
	default:
		from, to, err := ParseRangeString(c.Packets)
		if err != nil {
			return nil, errors.New("Invalid PacketsFrom").Base(err)
		}
		config.PacketsFrom = int64(from)
		config.PacketsTo = int64(to)
		if config.PacketsFrom == 0 {
			return nil, errors.New("PacketsFrom can't be 0")
		}
	}

	config.LengthMin = int64(c.Length.From)
	config.LengthMax = int64(c.Length.To)
	if config.LengthMin == 0 {
		return nil, errors.New("LengthMin can't be 0")
	}

	config.DelayMin = int64(c.Delay.From)
	config.DelayMax = int64(c.Delay.To)

	config.MaxSplitMin = int64(c.MaxSplit.From)
	config.MaxSplitMax = int64(c.MaxSplit.To)

	return config, nil
}

type NoiseItem struct {
	Rand      Int32Range      `json:"rand"`
	RandRange *Int32Range     `json:"randRange"`
	Type      string          `json:"type"`
	Packet    json.RawMessage `json:"packet"`
	Delay     Int32Range      `json:"delay"`
}

type NoiseMask struct {
	Reset Int32Range  `json:"reset"`
	Noise []NoiseItem `json:"noise"`
}

func (c *NoiseMask) Build() (proto.Message, error) {
	for _, item := range c.Noise {
		if len(item.Packet) > 0 && item.Rand.To > 0 {
			return nil, errors.New("len(item.Packet) > 0 && item.Rand.To > 0")
		}
	}

	noiseSlice := make([]*noise.Item, 0, len(c.Noise))
	for _, item := range c.Noise {
		if item.RandRange == nil {
			item.RandRange = &Int32Range{From: 0, To: 255}
		}
		if item.RandRange.From < 0 || item.RandRange.To > 255 {
			return nil, errors.New("invalid randRange")
		}
		var err error
		if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
			return nil, err
		}
		noiseSlice = append(noiseSlice, &noise.Item{
			RandMin:      int64(item.Rand.From),
			RandMax:      int64(item.Rand.To),
			RandRangeMin: item.RandRange.From,
			RandRangeMax: item.RandRange.To,
			Packet:       item.Packet,
			DelayMin:     int64(item.Delay.From),
			DelayMax:     int64(item.Delay.To),
		})
	}

	return &noise.Config{
		ResetMin: int64(c.Reset.From),
		ResetMax: int64(c.Reset.To),
		Items:    noiseSlice,
	}, nil
}

type UDPItem struct {
	Rand      int32           `json:"rand"`
	RandRange *Int32Range     `json:"randRange"`
	Type      string          `json:"type"`
	Packet    json.RawMessage `json:"packet"`
}

type HeaderCustomUDP struct {
	Client []UDPItem `json:"client"`
	Server []UDPItem `json:"server"`
}

func (c *HeaderCustomUDP) Build() (proto.Message, error) {
	for _, item := range c.Client {
		if len(item.Packet) > 0 && item.Rand > 0 {
			return nil, errors.New("len(item.Packet) > 0 && item.Rand > 0")
		}
	}
	for _, item := range c.Server {
		if len(item.Packet) > 0 && item.Rand > 0 {
			return nil, errors.New("len(item.Packet) > 0 && item.Rand > 0")
		}
	}

	client := make([]*custom.UDPItem, 0, len(c.Client))
	for _, item := range c.Client {
		if item.RandRange == nil {
			item.RandRange = &Int32Range{From: 0, To: 255}
		}
		if item.RandRange.From < 0 || item.RandRange.To > 255 {
			return nil, errors.New("invalid randRange")
		}
		var err error
		if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
			return nil, err
		}
		client = append(client, &custom.UDPItem{
			Rand:    item.Rand,
			RandMin: item.RandRange.From,
			RandMax: item.RandRange.To,
			Packet:  item.Packet,
		})
	}

	server := make([]*custom.UDPItem, 0, len(c.Server))
	for _, item := range c.Server {
		if item.RandRange == nil {
			item.RandRange = &Int32Range{From: 0, To: 255}
		}
		if item.RandRange.From < 0 || item.RandRange.To > 255 {
			return nil, errors.New("invalid randRange")
		}
		var err error
		if item.Packet, err = PraseByteSlice(item.Packet, item.Type); err != nil {
			return nil, err
		}
		server = append(server, &custom.UDPItem{
			Rand:    item.Rand,
			RandMin: item.RandRange.From,
			RandMax: item.RandRange.To,
			Packet:  item.Packet,
		})
	}

	return &custom.UDPConfig{
		Client: client,
		Server: server,
	}, nil
}

type Dns struct {
	Domain string `json:"domain"`
}

func (c *Dns) Build() (proto.Message, error) {
	config := &dns.Config{}
	config.Domain = "www.baidu.com"

	if len(c.Domain) > 0 {
		config.Domain = c.Domain
	}

	return config, nil
}

type Dtls struct{}

func (c *Dtls) Build() (proto.Message, error) {
	return &dtls.Config{}, nil
}

type Srtp struct{}

func (c *Srtp) Build() (proto.Message, error) {
	return &srtp.Config{}, nil
}

type Utp struct{}

func (c *Utp) Build() (proto.Message, error) {
	return &utp.Config{}, nil
}

type Wechat struct{}

func (c *Wechat) Build() (proto.Message, error) {
	return &wechat.Config{}, nil
}

type Wireguard struct{}

func (c *Wireguard) Build() (proto.Message, error) {
	return &wireguard.Config{}, nil
}

type Original struct{}

func (c *Original) Build() (proto.Message, error) {
	return &original.Config{}, nil
}

type Aes128Gcm struct {
	Password string `json:"password"`
}

func (c *Aes128Gcm) Build() (proto.Message, error) {
	return &aes128gcm.Config{
		Password: c.Password,
	}, nil
}

type Salamander struct {
	Password string `json:"password"`
}

func (c *Salamander) Build() (proto.Message, error) {
	config := &salamander.Config{}
	config.Password = c.Password
	return config, nil
}

type Sudoku struct {
	Password string `json:"password"`
	ASCII    string `json:"ascii"`

	CustomTable       string   `json:"customTable"`
	LegacyCustomTable string   `json:"custom_table"`
	CustomTables      []string `json:"customTables"`
	LegacyCustomSets  []string `json:"custom_tables"`

	PaddingMin       uint32 `json:"paddingMin"`
	LegacyPaddingMin uint32 `json:"padding_min"`
	PaddingMax       uint32 `json:"paddingMax"`
	LegacyPaddingMax uint32 `json:"padding_max"`
}

func (c *Sudoku) Build() (proto.Message, error) {
	customTable := c.CustomTable
	if customTable == "" {
		customTable = c.LegacyCustomTable
	}
	customTables := c.CustomTables
	if len(customTables) == 0 {
		customTables = c.LegacyCustomSets
	}

	paddingMin := c.PaddingMin
	if paddingMin == 0 {
		paddingMin = c.LegacyPaddingMin
	}
	paddingMax := c.PaddingMax
	if paddingMax == 0 {
		paddingMax = c.LegacyPaddingMax
	}

	return &finalsudoku.Config{
		Password:     c.Password,
		Ascii:        c.ASCII,
		CustomTable:  customTable,
		CustomTables: customTables,
		PaddingMin:   paddingMin,
		PaddingMax:   paddingMax,
	}, nil
}

type Xdns struct {
	Domain string `json:"domain"`
}

func (c *Xdns) Build() (proto.Message, error) {
	if c.Domain == "" {
		return nil, errors.New("empty domain")
	}

	return &xdns.Config{
		Domain: c.Domain,
	}, nil
}

type Xicmp struct {
	ListenIp string `json:"listenIp"`
	Id       uint16 `json:"id"`
}

func (c *Xicmp) Build() (proto.Message, error) {
	config := &xicmp.Config{
		Ip: c.ListenIp,
		Id: int32(c.Id),
	}

	if config.Ip == "" {
		config.Ip = "0.0.0.0"
	}

	return config, nil
}

type Mask struct {
	Type     string           `json:"type"`
	Settings *json.RawMessage `json:"settings"`
}

func (c *Mask) Build(tcp bool) (proto.Message, error) {
	loader := udpmaskLoader
	if tcp {
		loader = tcpmaskLoader
	}

	settings := []byte("{}")
	if c.Settings != nil {
		settings = ([]byte)(*c.Settings)
	}
	rawConfig, err := loader.LoadWithID(settings, c.Type)
	if err != nil {
		return nil, err
	}
	ts, err := rawConfig.(Buildable).Build()
	if err != nil {
		return nil, err
	}
	return ts, nil
}

type FinalMask struct {
	Tcp        []Mask            `json:"tcp"`
	Udp        []Mask            `json:"udp"`
	QuicParams *QuicParamsConfig `json:"quicParams"`
}

type StreamConfig struct {
	Address             *Address           `json:"address"`
	Port                uint16             `json:"port"`
	Network             *TransportProtocol `json:"network"`
	Security            string             `json:"security"`
	FinalMask           *FinalMask         `json:"finalmask"`
	TLSSettings         *TLSConfig         `json:"tlsSettings"`
	REALITYSettings     *REALITYConfig     `json:"realitySettings"`
	RAWSettings         *TCPConfig         `json:"rawSettings"`
	TCPSettings         *TCPConfig         `json:"tcpSettings"`
	XHTTPSettings       *SplitHTTPConfig   `json:"xhttpSettings"`
	SplitHTTPSettings   *SplitHTTPConfig   `json:"splithttpSettings"`
	KCPSettings         *KCPConfig         `json:"kcpSettings"`
	GRPCSettings        *GRPCConfig        `json:"grpcSettings"`
	WSSettings          *WebSocketConfig   `json:"wsSettings"`
	HTTPUPGRADESettings *HttpUpgradeConfig `json:"httpupgradeSettings"`
	HysteriaSettings    *HysteriaConfig    `json:"hysteriaSettings"`
	SocketSettings      *SocketConfig      `json:"sockopt"`
}

// Build implements Buildable.
func (c *StreamConfig) Build() (*internet.StreamConfig, error) {
	config := &internet.StreamConfig{
		Port:         uint32(c.Port),
		ProtocolName: "tcp",
	}
	if c.Address != nil {
		config.Address = c.Address.Build()
	}
	if c.Network != nil {
		protocol, err := c.Network.Build()
		if err != nil {
			return nil, err
		}
		config.ProtocolName = protocol
	}

	switch strings.ToLower(c.Security) {
	case "", "none":
	case "tls":
		tlsSettings := c.TLSSettings
		if tlsSettings == nil {
			tlsSettings = &TLSConfig{}
		}
		ts, err := tlsSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build TLS config.").Base(err)
		}
		tm := serial.ToTypedMessage(ts)
		config.SecuritySettings = append(config.SecuritySettings, tm)
		config.SecurityType = tm.Type
	case "reality":
		if config.ProtocolName != "tcp" && config.ProtocolName != "splithttp" && config.ProtocolName != "grpc" {
			return nil, errors.New("REALITY only supports RAW, XHTTP and gRPC for now.")
		}
		if c.REALITYSettings == nil {
			return nil, errors.New(`REALITY: Empty "realitySettings".`)
		}
		ts, err := c.REALITYSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build REALITY config.").Base(err)
		}
		tm := serial.ToTypedMessage(ts)
		config.SecuritySettings = append(config.SecuritySettings, tm)
		config.SecurityType = tm.Type
	case "xtls":
		return nil, errors.PrintRemovedFeatureError(`Legacy XTLS`, `xtls-rprx-vision with TLS or REALITY`)
	default:
		return nil, errors.New(`Unknown security "` + c.Security + `".`)
	}

	if c.RAWSettings != nil {
		c.TCPSettings = c.RAWSettings
	}
	if c.TCPSettings != nil {
		ts, err := c.TCPSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build RAW config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "tcp",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.XHTTPSettings != nil {
		c.SplitHTTPSettings = c.XHTTPSettings
	}
	if c.SplitHTTPSettings != nil {
		hs, err := c.SplitHTTPSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build XHTTP config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "splithttp",
			Settings:     serial.ToTypedMessage(hs),
		})
	}
	if c.KCPSettings != nil {
		ts, err := c.KCPSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build mKCP config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "mkcp",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.GRPCSettings != nil {
		gs, err := c.GRPCSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build gRPC config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "grpc",
			Settings:     serial.ToTypedMessage(gs),
		})
	}
	if c.WSSettings != nil {
		ts, err := c.WSSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build WebSocket config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "websocket",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.HTTPUPGRADESettings != nil {
		hs, err := c.HTTPUPGRADESettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build HTTPUpgrade config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "httpupgrade",
			Settings:     serial.ToTypedMessage(hs),
		})
	}
	if c.HysteriaSettings != nil {
		hs, err := c.HysteriaSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build Hysteria config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "hysteria",
			Settings:     serial.ToTypedMessage(hs),
		})
	}
	if c.SocketSettings != nil {
		ss, err := c.SocketSettings.Build()
		if err != nil {
			return nil, errors.New("Failed to build sockopt.").Base(err)
		}
		config.SocketSettings = ss
	}

	if c.FinalMask != nil {
		for _, mask := range c.FinalMask.Tcp {
			u, err := mask.Build(true)
			if err != nil {
				return nil, errors.New("failed to build mask with type ", mask.Type).Base(err)
			}
			config.Tcpmasks = append(config.Tcpmasks, serial.ToTypedMessage(u))
		}
		for _, mask := range c.FinalMask.Udp {
			u, err := mask.Build(false)
			if err != nil {
				return nil, errors.New("failed to build mask with type ", mask.Type).Base(err)
			}
			config.Udpmasks = append(config.Udpmasks, serial.ToTypedMessage(u))
		}
		if c.FinalMask.QuicParams != nil {
			profile := strings.ToLower(c.FinalMask.QuicParams.BbrProfile)
			switch profile {
			case "", string(bbr.ProfileConservative), string(bbr.ProfileStandard), string(bbr.ProfileAggressive):
				if profile == "" {
					profile = string(bbr.ProfileStandard)
				}
			default:
				return nil, errors.New("unknown bbr profile")
			}

			up, err := c.FinalMask.QuicParams.BrutalUp.Bps()
			if err != nil {
				return nil, err
			}
			down, err := c.FinalMask.QuicParams.BrutalDown.Bps()
			if err != nil {
				return nil, err
			}

			if up > 0 && up < 65536 {
				return nil, errors.New("BrutalUp must be at least 65536 bytes per second")
			}
			if down > 0 && down < 65536 {
				return nil, errors.New("BrutalDown must be at least 65536 bytes per second")
			}

			c.FinalMask.QuicParams.Congestion = strings.ToLower(c.FinalMask.QuicParams.Congestion)
			switch c.FinalMask.QuicParams.Congestion {
			case "", "brutal", "reno", "bbr":
			case "force-brutal":
				if up == 0 {
					return nil, errors.New("force-brutal requires up")
				}
			default:
				return nil, errors.New("unknown congestion control: ", c.FinalMask.QuicParams.Congestion, ", valid values: reno, bbr, brutal, force-brutal")
			}

			var hop *PortList
			if err := json.Unmarshal(c.FinalMask.QuicParams.UdpHop.PortList, &hop); err != nil {
				hop = &PortList{}
			}

			var inertvalMin, inertvalMax int64
			if c.FinalMask.QuicParams.UdpHop.Interval != nil {
				inertvalMin = int64(c.FinalMask.QuicParams.UdpHop.Interval.From)
				inertvalMax = int64(c.FinalMask.QuicParams.UdpHop.Interval.To)
			}

			if (inertvalMin != 0 && inertvalMin < 5) || (inertvalMax != 0 && inertvalMax < 5) {
				return nil, errors.New("Interval must be at least 5")
			}

			if c.FinalMask.QuicParams.InitStreamReceiveWindow > 0 && c.FinalMask.QuicParams.InitStreamReceiveWindow < 16384 {
				return nil, errors.New("InitStreamReceiveWindow must be at least 16384")
			}
			if c.FinalMask.QuicParams.MaxStreamReceiveWindow > 0 && c.FinalMask.QuicParams.MaxStreamReceiveWindow < 16384 {
				return nil, errors.New("MaxStreamReceiveWindow must be at least 16384")
			}
			if c.FinalMask.QuicParams.InitConnectionReceiveWindow > 0 && c.FinalMask.QuicParams.InitConnectionReceiveWindow < 16384 {
				return nil, errors.New("InitConnectionReceiveWindow must be at least 16384")
			}
			if c.FinalMask.QuicParams.MaxConnectionReceiveWindow > 0 && c.FinalMask.QuicParams.MaxConnectionReceiveWindow < 16384 {
				return nil, errors.New("MaxConnectionReceiveWindow must be at least 16384")
			}
			if c.FinalMask.QuicParams.MaxIdleTimeout != 0 && (c.FinalMask.QuicParams.MaxIdleTimeout < 4 || c.FinalMask.QuicParams.MaxIdleTimeout > 120) {
				return nil, errors.New("MaxIdleTimeout must be between 4 and 120")
			}
			if c.FinalMask.QuicParams.KeepAlivePeriod != 0 && (c.FinalMask.QuicParams.KeepAlivePeriod < 2 || c.FinalMask.QuicParams.KeepAlivePeriod > 60) {
				return nil, errors.New("KeepAlivePeriod must be between 2 and 60")
			}
			if c.FinalMask.QuicParams.MaxIncomingStreams != 0 && c.FinalMask.QuicParams.MaxIncomingStreams < 8 {
				return nil, errors.New("MaxIncomingStreams must be at least 8")
			}

			if c.FinalMask.QuicParams.Debug {
				os.Setenv("HYSTERIA_BBR_DEBUG", "true")
				os.Setenv("HYSTERIA_BRUTAL_DEBUG", "true")
			}

			config.QuicParams = &internet.QuicParams{
				Congestion: c.FinalMask.QuicParams.Congestion,
				BbrProfile: profile,
				BrutalUp:   up,
				BrutalDown: down,
				UdpHop: &internet.UdpHop{
					Ports:       hop.Build().Ports(),
					IntervalMin: inertvalMin,
					IntervalMax: inertvalMax,
				},
				InitStreamReceiveWindow: c.FinalMask.QuicParams.InitStreamReceiveWindow,
				MaxStreamReceiveWindow:  c.FinalMask.QuicParams.MaxStreamReceiveWindow,
				InitConnReceiveWindow:   c.FinalMask.QuicParams.InitConnectionReceiveWindow,
				MaxConnReceiveWindow:    c.FinalMask.QuicParams.MaxConnectionReceiveWindow,
				MaxIdleTimeout:          c.FinalMask.QuicParams.MaxIdleTimeout,
				KeepAlivePeriod:         c.FinalMask.QuicParams.KeepAlivePeriod,
				DisablePathMtuDiscovery: c.FinalMask.QuicParams.DisablePathMTUDiscovery,
				MaxIncomingStreams:      c.FinalMask.QuicParams.MaxIncomingStreams,
			}
		}
	}

	return config, nil
}

type ProxyConfig struct {
	Tag string `json:"tag"`

	// TransportLayerProxy: For compatibility.
	TransportLayerProxy bool `json:"transportLayer"`
}

// Build implements Buildable.
func (v *ProxyConfig) Build() (*internet.ProxyConfig, error) {
	if v.Tag == "" {
		return nil, errors.New("Proxy tag is not set.")
	}
	return &internet.ProxyConfig{
		Tag:                 v.Tag,
		TransportLayerProxy: v.TransportLayerProxy,
	}, nil
}
