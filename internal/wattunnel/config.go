package wattunnel

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
)

const Version = "3.3.0"

var carrierOrder = []string{"udp", "tcp", "tls", "wss", "icmp"}

type Port struct {
	Listen   int    `json:"listen" example:"8443"`
	Target   int    `json:"target" example:"443"`
	Protocol string `json:"protocol" example:"tcp"`
}

type CarrierEndpoint struct {
	Carrier string `json:"carrier" example:"tcp"`
	Port    int    `json:"port" example:"4443"`
}

type Config struct {
	Role        string            `json:"role" example:"client"`
	Carrier     string            `json:"carrier" example:"tcp"`
	Endpoint    string            `json:"endpoint" example:"203.0.113.10"`
	Port        int               `json:"port" example:"4443"`
	Reverse     bool              `json:"reverse"`
	ReversePeer string            `json:"reversePeer"`
	Bind        string            `json:"bind" example:"0.0.0.0"`
	Listen      string            `json:"listen" example:"0.0.0.0"`
	Ports       []Port            `json:"ports"`
	Candidates  []CarrierEndpoint `json:"candidates"`
	SNI         string            `json:"sni"`
	Path        string            `json:"path" example:"/wat"`
	TLSPin      string            `json:"tlsPin"`
	TLSCert     string            `json:"tlsCert"`
	TLSKey      string            `json:"tlsKey"`
	MTU         int               `json:"mtu"`
	Sessions    int               `json:"sessions"`
	Conns       int               `json:"conns"`
	FEC         bool              `json:"fec"`
	MemoryMB    int               `json:"memoryMB"`
	ICMPType    string            `json:"icmpType"`
}

func (c Config) Normalized() Config {
	if c.Bind == "" {
		c.Bind = "0.0.0.0"
	}
	if c.Listen == "" {
		c.Listen = "0.0.0.0"
	}
	if c.Path == "" {
		c.Path = "/wat"
	}
	if c.ICMPType == "" {
		c.ICMPType = "echo"
	}
	if c.Ports == nil {
		c.Ports = []Port{}
	}
	if c.Carrier == "auto" && len(c.Candidates) == 0 {
		c.Candidates = []CarrierEndpoint{{"udp", 4443}, {"tcp", 4443}, {"tls", 8443}, {"wss", 2054}, {"icmp", 0}}
	}
	if c.Candidates == nil {
		c.Candidates = []CarrierEndpoint{}
	}
	return c
}

func cleanScalar(s string) bool { return !strings.ContainsAny(s, "\r\n\x00|\"'\t#") && len(s) <= 1024 }

func validHost(s string) bool {
	if s == "" || !cleanScalar(s) || strings.ContainsAny(s, " /\\@,;=") {
		return false
	}
	if net.ParseIP(s) != nil {
		return true
	}
	if len(s) > 253 || strings.Contains(s, ":") {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(s, "."), ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}
func validPort(p int) bool { return p > 0 && p <= 65535 }

func (c Config) Validate() error {
	c = c.Normalized()
	if c.Role != "client" && c.Role != "server" {
		return errors.New("role must be client or server")
	}
	if c.Carrier != "auto" && !slices.Contains(carrierOrder, c.Carrier) {
		return errors.New("unsupported WatTunnel carrier")
	}
	if !validHost(c.Endpoint) {
		return errors.New("invalid endpoint host")
	}
	if c.ReversePeer != "" && !validHost(c.ReversePeer) {
		return errors.New("invalid reverse peer host")
	}
	if c.Reverse && c.Role == "server" && c.ReversePeer == "" {
		return errors.New("reverse server requires the client address")
	}
	if net.ParseIP(c.Bind) == nil || net.ParseIP(c.Listen) == nil {
		return errors.New("bind and listen must be IP addresses")
	}
	if c.Carrier != "icmp" && c.Carrier != "auto" && !validPort(c.Port) {
		return errors.New("carrier port must be 1..65535")
	}
	if len(c.Ports) == 0 || len(c.Ports) > 128 {
		return errors.New("specify 1..128 allowed port mappings")
	}
	seen := make(map[string]bool)
	for _, p := range c.Ports {
		if !validPort(p.Listen) || !validPort(p.Target) || (p.Protocol != "tcp" && p.Protocol != "udp") {
			return errors.New("invalid TCP/UDP port mapping")
		}
		key := fmt.Sprintf("%s/%d", p.Protocol, p.Listen)
		if seen[key] {
			return errors.New("duplicate listen port for the same protocol")
		}
		seen[key] = true
	}
	if len(c.Candidates) > 5 {
		return errors.New("too many Auto candidates")
	}
	seen = make(map[string]bool)
	for _, p := range c.Candidates {
		if !slices.Contains(carrierOrder, p.Carrier) || seen[p.Carrier] || (p.Carrier != "icmp" && !validPort(p.Port)) {
			return errors.New("invalid or duplicate Auto candidate")
		}
		seen[p.Carrier] = true
	}
	for _, s := range []string{c.Path, c.SNI, c.TLSPin, c.TLSCert, c.TLSKey} {
		if !cleanScalar(s) {
			return errors.New("invalid configuration scalar")
		}
	}
	if c.SNI != "" && !validHost(c.SNI) {
		return errors.New("invalid SNI")
	}
	if !strings.HasPrefix(c.Path, "/") {
		return errors.New("WebSocket path must start with /")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return errors.New("TLS certificate and key must be supplied together")
	}
	if c.TLSPin != "" {
		b, err := hex.DecodeString(c.TLSPin)
		if err != nil || len(b) != 32 {
			return errors.New("TLS pin must be 64 hexadecimal characters")
		}
	}
	maxMTU := 16384
	if c.Carrier == "icmp" || c.Carrier == "udp" || c.Carrier == "auto" {
		maxMTU = 1400
	}
	if c.MTU != 0 && (c.MTU < 256 || c.MTU > maxMTU) {
		return errors.New("MTU outside carrier bounds")
	}
	if c.Sessions < 0 || c.Sessions > 16 || c.Conns < 0 || c.Conns > 64 || c.MemoryMB < 0 || c.MemoryMB > 65536 {
		return errors.New("resource setting outside supported bounds")
	}
	if c.Sessions > 1 && (c.Reverse || c.Carrier == "icmp" || c.Role == "server") {
		return errors.New("multiple sessions require a direct non-ICMP client")
	}
	if c.ICMPType != "echo" && c.ICMPType != "timestamp" {
		return errors.New("unsupported ICMP type")
	}
	return nil
}

func NewKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ValidateKey(key string) error {
	if len(key) < 1 || len(key) > 1024 || strings.TrimSpace(key) != key || !cleanScalar(key) {
		return errors.New("invalid tunnel key")
	}
	return nil
}

func (c Config) CarrierPort(carrier string) int {
	if c.Carrier == "auto" {
		for _, p := range c.Normalized().Candidates {
			if p.Carrier == carrier {
				return p.Port
			}
		}
	}
	return c.Port
}

func Render(c Config, keyPath, metrics, selected string, resolvedICMPPeer ...string) (string, error) {
	c = c.Normalized()
	if err := c.Validate(); err != nil {
		return "", err
	}
	if !cleanScalar(keyPath) {
		return "", errors.New("invalid key path")
	}
	host, port, err := net.SplitHostPort(metrics)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return "", errors.New("metrics must use loopback")
	}
	mp, err := strconv.Atoi(port)
	if err != nil || !validPort(mp) {
		return "", errors.New("invalid metrics port")
	}
	names := []string{selected}
	if c.Carrier == "auto" && c.Role == "server" {
		names = nil
		for _, p := range c.Candidates {
			names = append(names, p.Carrier)
		}
	}
	for _, name := range names {
		if !slices.Contains(carrierOrder, name) {
			return "", errors.New("unsupported selected carrier")
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "mode: %s\ncarrier: %s\nkey-file: %s\nmetrics: %s\nbind: %s\nreverse: %t\npath: %s\nicmp-type: %s\n", c.Role, strings.Join(names, "+"), keyPath, metrics, c.Bind, c.Reverse, c.Path, c.ICMPType)
	peer := c.Endpoint
	if c.Reverse && c.Role == "server" {
		peer = c.ReversePeer
	}
	for _, name := range names {
		suffix := ""
		if len(names) > 1 {
			suffix = "-" + name
		}
		if name == "icmp" {
			icmpPeer := peer
			if len(resolvedICMPPeer) > 0 {
				icmpPeer = resolvedICMPPeer[0]
			}
			fmt.Fprintf(&b, "peer%s: %s\n", suffix, icmpPeer)
			continue
		}
		p := c.CarrierPort(name)
		if c.Role == "client" && !c.Reverse || c.Role == "server" && c.Reverse {
			fmt.Fprintf(&b, "peer%s: %s\n", suffix, net.JoinHostPort(peer, strconv.Itoa(p)))
		} else {
			fmt.Fprintf(&b, "listen%s: %s\n", suffix, net.JoinHostPort(c.Listen, strconv.Itoa(p)))
		}
	}
	for _, v := range []struct{ k, v string }{{"sni", c.SNI}, {"tls-pin", c.TLSPin}, {"tls-cert", c.TLSCert}, {"tls-key", c.TLSKey}} {
		if v.v != "" {
			fmt.Fprintf(&b, "%s: %s\n", v.k, v.v)
		}
	}
	sessions := c.Sessions
	if selected == "icmp" || c.Reverse || c.Role == "server" {
		sessions = 0
	}
	fmt.Fprintf(&b, "mtu: %d\nsessions: %d\nconns: %d\nmem-budget: %d\nfec: %s\nports:\n", c.MTU, sessions, c.Conns, c.MemoryMB, map[bool]string{true: "auto", false: "off"}[c.FEC])
	seen := make(map[string]bool)
	for _, p := range c.Ports {
		item := fmt.Sprintf("%d:%d/%s", p.Listen, p.Target, p.Protocol)
		if c.Role == "server" {
			item = fmt.Sprintf("%d/%s", p.Target, p.Protocol)
		}
		if p.Target == mp {
			return "", errors.New("target port conflicts with metrics")
		}
		if !seen[item] {
			fmt.Fprintf(&b, "  - %s\n", item)
			seen[item] = true
		}
	}
	return b.String(), nil
}

func EncodeToken(c Config, key string) (string, error) {
	c = c.Normalized()
	if err := c.Validate(); err != nil {
		return "", err
	}
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	peer := c.Endpoint
	if c.Carrier != "icmp" && c.Carrier != "auto" {
		peer = net.JoinHostPort(peer, strconv.Itoa(c.Port))
	}
	ports := []string{}
	seen := map[string]bool{}
	for _, p := range c.Ports {
		v := fmt.Sprintf("%d/%s", p.Target, p.Protocol)
		if !seen[v] {
			ports = append(ports, v)
			seen[v] = true
		}
	}
	fields := []string{c.Carrier, peer, key, c.Path, c.SNI, c.ICMPType, strings.Join(ports, ",")}
	if c.Reverse || c.Carrier == "auto" {
		v := ""
		if c.Reverse {
			v = "1"
		}
		fields = append(fields, v)
	}
	if c.Carrier == "auto" {
		plan := []string{}
		for _, p := range c.Candidates {
			v := p.Carrier
			if v != "icmp" {
				v += ":" + strconv.Itoa(p.Port)
			}
			plan = append(plan, v)
		}
		direction := "d="
		if c.Reverse {
			direction = "r="
		}
		fields = append(fields, direction+strings.Join(plan, ","))
	}
	return "wat1:" + base64.StdEncoding.EncodeToString([]byte(strings.Join(fields, "|"))), nil
}

func DecodeToken(raw string) (Config, string, error) {
	fail := func() (Config, string, error) {
		return Config{}, "", errors.New("invalid or unsupported WatTunnel token")
	}
	if len(raw) > 16384 || !strings.HasPrefix(raw, "wat1:") {
		return fail()
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, "wat1:"))
	if err != nil {
		return fail()
	}
	f := strings.Split(string(b), "|")
	if len(f) < 7 || len(f) > 9 {
		return fail()
	}
	if err := ValidateKey(f[2]); err != nil {
		return fail()
	}
	c := Config{Role: "client", Carrier: f[0], Endpoint: f[1], Path: f[3], SNI: f[4], ICMPType: f[5]}
	if len(f) > 7 {
		if f[7] != "" && f[7] != "1" {
			return fail()
		}
		c.Reverse = f[7] == "1"
	}
	if c.Carrier != "icmp" && c.Carrier != "auto" {
		host, p, e := net.SplitHostPort(c.Endpoint)
		if e != nil {
			return fail()
		}
		c.Endpoint = host
		c.Port, err = strconv.Atoi(p)
		if err != nil {
			return fail()
		}
	}
	for _, v := range strings.Split(f[6], ",") {
		proto := "tcp"
		n := v
		if a, z, ok := strings.Cut(v, "/"); ok {
			n = a
			proto = z
		}
		p, e := strconv.Atoi(n)
		if e != nil {
			return fail()
		}
		c.Ports = append(c.Ports, Port{p, p, proto})
	}
	if c.Carrier == "auto" {
		if len(f) != 9 {
			return fail()
		}
		want := "d="
		if c.Reverse {
			want = "r="
		}
		for _, part := range strings.Split(f[8], ";") {
			if !strings.HasPrefix(part, want) {
				continue
			}
			for _, item := range strings.Split(strings.TrimPrefix(part, want), ",") {
				name, p, _ := strings.Cut(item, ":")
				if !slices.Contains(carrierOrder, name) {
					continue
				}
				port := 0
				if name != "icmp" {
					port, err = strconv.Atoi(p)
					if err != nil {
						return fail()
					}
				}
				c.Candidates = append(c.Candidates, CarrierEndpoint{name, port})
			}
		}
		if len(c.Candidates) == 0 {
			return fail()
		}
	}
	c = c.Normalized()
	if err := c.Validate(); err != nil {
		return fail()
	}
	return c, f[2], nil
}
