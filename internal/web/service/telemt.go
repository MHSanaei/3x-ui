package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

var telemtLatestCache struct {
	sync.Mutex
	latest  string
	checked time.Time
	current string
}

const (
	defaultTelemtSNI = "petrovich.ru"

	telemtConfigPath      = "/etc/x-ui/telemt.toml"
	telemtServiceName     = "telemt.service"
	telemtMekoServiceName = "telemt-meko-fix.service"
	telemtBinaryPath      = "/usr/local/x-ui/bin/telemt"
)

type TelemtConfig struct {
	Enabled      bool   `json:"enabled"`
	Port         int    `json:"port"`
	Secret       string `json:"secret"`
	IPv4         bool   `json:"ipv4"`
	IPv6         bool   `json:"ipv6"`
	FastMode     bool   `json:"fastMode"`
	Classic      bool   `json:"classic"`
	Secure       bool   `json:"secure"`
	TLS          bool   `json:"tls"`
	SNI          string `json:"sni"`
	UpstreamType string `json:"upstreamType"`
}

type TelemtStatus struct {
	Installed  bool   `json:"installed"`
	Active     bool   `json:"active"`
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
	Version    string `json:"version"`
	LatestVersion string `json:"latestVersion"`
	UpdateAvailable bool `json:"updateAvailable"`
	MekoEnabled bool `json:"mekoEnabled"`
}

type TelemtProxy struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	TLS    bool   `json:"tls"`
	Link   string `json:"link"`
}

type TelemtCreateRequest struct {
	Name string `json:"name"`
	Host string `json:"host"`
}

type TelemtService struct{}

func defaultTelemtConfig() TelemtConfig {
	return TelemtConfig{Port: 8443, IPv4: true, IPv6: true, FastMode: true, TLS: true, UpstreamType: "direct"}
}

func renderTelemtConfig(c TelemtConfig) (string, error) {
	if c.Port < 1 || c.Port > 65535 {
		return "", errors.New("telemt: invalid port")
	}
	if !c.IPv4 && !c.IPv6 {
		return "", errors.New("telemt: enable IPv4 or IPv6")
	}
	if len(c.Secret) != 32 {
		return "", errors.New("telemt: secret must contain exactly 32 hexadecimal characters")
	}
	if _, err := hex.DecodeString(c.Secret); err != nil {
		return "", errors.New("telemt: secret must be hexadecimal")
	}
	if c.UpstreamType != "direct" {
		return "", errors.New("telemt: only direct upstream is supported by the panel")
	}

	sni := strings.TrimSpace(c.SNI)
	if c.TLS && sni == "" {
		return "", errors.New("telemt: SNI is required when Fake-TLS is enabled")
	}
	if strings.ContainsAny(sni, "\"\r\n\t ") {
		return "", errors.New("telemt: invalid SNI")
	}

	listeners := ""
	if c.IPv4 {
		listeners += "[[server.listeners]]\nip = \"0.0.0.0\"\n\n"
	}
	if c.IPv6 {
		listeners += "[[server.listeners]]\nip = \"::\"\n\n"
	}

	censorship := ""
	if c.TLS {
		censorship = fmt.Sprintf("[censorship]\ntls_domain = \"%s\"\nmask = true\ntls_emulation = true\ntls_front_dir = \"tlsfront\"\n\n", sni)
	}

	return fmt.Sprintf("[general]\nfast_mode = %t\nuse_middle_proxy = false\nlog_level = \"normal\"\n\n[general.modes]\nclassic = %t\nsecure = %t\ntls = %t\n\n[general.links]\nshow = \"*\"\n\n[server.api]\nenabled = true\nlisten = \"127.0.0.1:9091\"\nwhitelist = [\"127.0.0.1/32\", \"::1/128\"]\nread_only = false\n\n[network]\nipv4 = %t\nipv6 = %t\n\n[server]\nport = %d\n\n%s%s[access]\nreplay_check_len = 65536\nignore_time_skew = false\n\n[access.users]\nxui = \"%s\"\n\n[[upstreams]]\ntype = \"direct\"\nweight = 1\nenabled = true\n", c.FastMode, c.Classic, c.Secure, c.TLS, c.IPv4, c.IPv6, c.Port, listeners, censorship, strings.ToLower(c.Secret)), nil
}

func ensureTelemtSNI() error {
	b, err := os.ReadFile(telemtConfigPath)
	if err != nil {
		return err
	}

	var raw struct {
		General struct {
			Modes struct {
				TLS bool `toml:"tls"`
			} `toml:"modes"`
		} `toml:"general"`
		Censorship struct {
			TLSDomain string `toml:"tls_domain"`
		} `toml:"censorship"`
	}
	if err := toml.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("telemt: parse config: %w", err)
	}
	if !raw.General.Modes.TLS || strings.TrimSpace(raw.Censorship.TLSDomain) != "" {
		return nil
	}

	text := string(b)
	const section = "[censorship]"
	if idx := strings.Index(text, section); idx >= 0 {
		sectionStart := idx + len(section)
		next := strings.Index(text[sectionStart:], "\n[")
		insertAt := len(text)
		if next >= 0 {
			insertAt = sectionStart + next + 1
		}
		entry := fmt.Sprintf("\ntls_domain = \"%s\"\n", defaultTelemtSNI)
		text = text[:insertAt] + entry + text[insertAt:]
	} else {
		text = strings.TrimRight(text, "\n") + fmt.Sprintf("\n\n[censorship]\ntls_domain = \"%s\"\nmask = true\ntls_emulation = true\ntls_front_dir = \"tlsfront\"\n", defaultTelemtSNI)
	}
	if err := os.WriteFile(telemtConfigPath, []byte(text), 0600); err != nil {
		return fmt.Errorf("telemt: apply default SNI: %w", err)
	}
	return nil
}

func ensureTelemtConfig() error {
	if err := os.MkdirAll(filepath.Dir(telemtConfigPath), 0700); err != nil {
		return fmt.Errorf("telemt: create config directory: %w", err)
	}
	if _, err := os.Stat(telemtConfigPath); err == nil {
		return ensureTelemtSNI()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		return fmt.Errorf("telemt: generate secret: %w", err)
	}
	c := defaultTelemtConfig()
	c.Secret = hex.EncodeToString(secretBytes)
	c.SNI = defaultTelemtSNI
	data, err := renderTelemtConfig(c)
	if err != nil {
		return err
	}
	if err := os.WriteFile(telemtConfigPath, []byte(data), 0600); err != nil {
		return fmt.Errorf("telemt: create config: %w", err)
	}
	return nil
}

func (TelemtService) Status() TelemtStatus {
	_, binErr := os.Stat(telemtBinaryPath)
	_, cfgErr := os.Stat(telemtConfigPath)
	version := ""
	if binErr == nil {
		version = telemtVersion()
	}
	latest, available := telemtLatestVersion(version)
	return TelemtStatus{Installed: binErr == nil, Active: systemctl("is-active", "--quiet", telemtServiceName) == nil, Enabled: systemctl("is-enabled", "--quiet", telemtServiceName) == nil, Configured: cfgErr == nil, Version: version, LatestVersion: latest, UpdateAvailable: available, MekoEnabled: systemctl("is-enabled", "--quiet", telemtMekoServiceName) == nil}
}

func telemtLatestVersion(current string) (string, bool) {
	if _, err := os.Stat("/usr/local/x-ui/telemt-update.sh"); err != nil {
		return "", false
	}

	telemtLatestCache.Lock()
	defer telemtLatestCache.Unlock()

	// The updater performs a network request. Do not run it on every panel refresh.
	if telemtLatestCache.latest != "" && telemtLatestCache.current == current && time.Since(telemtLatestCache.checked) < 10*time.Minute {
		latest := telemtLatestCache.latest
		return latest, current != "" && strings.TrimPrefix(current, "v") != strings.TrimPrefix(latest, "v")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "/usr/local/x-ui/telemt-update.sh", "--check").CombinedOutput()
	latest := ""
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "latest=") {
			latest = strings.TrimSpace(strings.TrimPrefix(line, "latest="))
			break
		}
	}
	if latest == "" {
		return "", false
	}

	telemtLatestCache.latest = latest
	telemtLatestCache.checked = time.Now()
	telemtLatestCache.current = current
	return latest, current != "" && strings.TrimPrefix(current, "v") != strings.TrimPrefix(latest, "v")
}

func telemtUpdate() error {
	if _, err := os.Stat("/usr/local/x-ui/telemt-update.sh"); err != nil { return errors.New("telemt: updater is not installed") }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/local/x-ui/telemt-update.sh").CombinedOutput()
	if err != nil { return fmt.Errorf("telemt: update failed: %s: %w", strings.TrimSpace(string(out)), err) }
	return nil
}

func telemtVersion() string {
	for _, arg := range []string{"--version", "-V"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, err := exec.CommandContext(ctx, telemtBinaryPath, arg).CombinedOutput()
		cancel()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// fix by meko
func (TelemtService) GetConfig() (TelemtConfig, error) {
	if err := ensureTelemtConfig(); err != nil { return TelemtConfig{}, err }
	b, err := os.ReadFile(telemtConfigPath)
	if err != nil { return TelemtConfig{}, err }

	// Parse the real TOML structure so panel values match the file on disk.
	var raw struct {
		General struct {
			FastMode bool `toml:"fast_mode"`
			Modes struct {
				Classic bool `toml:"classic"`
				Secure bool `toml:"secure"`
				TLS bool `toml:"tls"`
			} `toml:"modes"`
		} `toml:"general"`
		Network struct {
			IPv4 bool `toml:"ipv4"`
			IPv6 bool `toml:"ipv6"`
		} `toml:"network"`
		Server struct { Port int `toml:"port"` } `toml:"server"`
		Censorship struct { TLSDomain string `toml:"tls_domain"` } `toml:"censorship"`
		Access struct { Users map[string]string `toml:"users"` } `toml:"access"`
		Upstreams []struct { Type string `toml:"type"` } `toml:"upstreams"`
	}
	if err := toml.Unmarshal(b, &raw); err != nil { return TelemtConfig{}, fmt.Errorf("telemt: parse config: %w", err) }

	c := defaultTelemtConfig()
	c.Port = raw.Server.Port
	c.IPv4 = raw.Network.IPv4
	c.IPv6 = raw.Network.IPv6
	c.FastMode = raw.General.FastMode
	c.Classic = raw.General.Modes.Classic
	c.Secure = raw.General.Modes.Secure
	c.TLS = raw.General.Modes.TLS
	c.SNI = raw.Censorship.TLSDomain
	c.Secret = strings.TrimSpace(raw.Access.Users["xui"])
	// Upstream selection is not exposed by the panel; always report the supported direct mode.\n\tc.UpstreamType = "direct"
	c.Enabled = systemctl("is-enabled", "--quiet", telemtServiceName) == nil

	// Prefer Telemt live configuration while the service is running. The TOML
	// file remains the fallback for fields hidden by /v1/config.
	if c.Enabled && systemctl("is-active", "--quiet", telemtServiceName) == nil {
		if live, err := fetchTelemtRuntimeConfig(); err == nil {
			if live.General.FastMode != nil { c.FastMode = *live.General.FastMode }
			if live.General.Modes.Classic != nil { c.Classic = *live.General.Modes.Classic }
			if live.General.Modes.Secure != nil { c.Secure = *live.General.Modes.Secure }
			if live.General.Modes.TLS != nil { c.TLS = *live.General.Modes.TLS }
			if live.Censorship.TLSDomain != "" { c.SNI = live.Censorship.TLSDomain }
		}
	}
	return c, nil
}

type telemtRuntimeConfigResponse struct {
	General struct {
		FastMode *bool `json:"fast_mode"`
		Modes struct {
			Classic *bool `json:"classic"`
			Secure *bool `json:"secure"`
			TLS *bool `json:"tls"`
		} `json:"modes"`
	} `json:"general"`
	Censorship struct { TLSDomain string `json:"tls_domain"` } `json:"censorship"`
}

func fetchTelemtRuntimeConfig() (telemtRuntimeConfigResponse, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:9091/v1/config", nil)
	if err != nil {
		return telemtRuntimeConfigResponse{}, err
	}
	resp, err := client.Do(req)
	if err != nil { return telemtRuntimeConfigResponse{}, err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return telemtRuntimeConfigResponse{}, fmt.Errorf("telemt: config API returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		OK bool `json:"ok"`
		Data telemtRuntimeConfigResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return telemtRuntimeConfigResponse{}, fmt.Errorf("telemt: decode runtime config: %w", err)
	}
	if !envelope.OK { return telemtRuntimeConfigResponse{}, errors.New("telemt: runtime config API rejected request") }
	return envelope.Data, nil
}
func (TelemtService) SaveConfig(c TelemtConfig) error {
	data, err := renderTelemtConfig(c)
	if err != nil {
		return err
	}
	if err := ensureTelemtConfig(); err != nil {
		return err
	}
	old, oldErr := os.ReadFile(telemtConfigPath)
	wasActive := systemctl("is-active", "--quiet", telemtServiceName) == nil
	if err := os.WriteFile(telemtConfigPath, []byte(data), 0600); err != nil {
		return err
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	if wasActive {
		if err := systemctl("restart", telemtServiceName); err != nil {
			if oldErr == nil {
				_ = os.WriteFile(telemtConfigPath, old, 0600)
				_ = systemctl("daemon-reload")
				_ = systemctl("restart", telemtServiceName)
			}
			return fmt.Errorf("telemt: new configuration was rejected: %w", err)
		}
	}
	return nil
}

func (TelemtService) CreateProxy(req TelemtCreateRequest) (TelemtProxy, error) {
	name := strings.TrimSpace(req.Name)
	host := strings.TrimSpace(req.Host)
	if name == "" {
		return TelemtProxy{}, errors.New("telemt: proxy name is required")
	}
	if host == "" {
		return TelemtProxy{}, errors.New("telemt: public host is required")
	}
	if strings.ContainsAny(name, "=\n\r\"") || strings.ContainsAny(host, " \t\n\r\"") {
		return TelemtProxy{}, errors.New("telemt: invalid proxy name or host")
	}
	if err := ensureTelemtConfig(); err != nil {
		return TelemtProxy{}, err
	}
	b, err := os.ReadFile(telemtConfigPath)
	if err != nil {
		return TelemtProxy{}, err
	}

	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		return TelemtProxy{}, err
	}
	secret := hex.EncodeToString(secretBytes)

	username := strings.ToLower(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return '_'
	}, name))
	username = strings.Trim(username, "_-")
	if username == "" {
		username = "proxy"
	}

	text := string(b)
	section := "[access.users]"
	idx := strings.Index(text, section)
	if idx < 0 {
		return TelemtProxy{}, errors.New("telemt: access.users section is missing")
	}
	insertAt := len(text)
	if next := strings.Index(text[idx+len(section):], "\n["); next >= 0 {
		insertAt = idx + len(section) + next + 1
	}
	for i := 2; strings.Contains(text[idx:insertAt], username+" = "); i++ {
		username = fmt.Sprintf("%s-%d", username, i)
	}

	entry := fmt.Sprintf("%s = \"%s\"\n", username, secret)
	old := append([]byte(nil), b...)
	text = text[:insertAt] + entry + text[insertAt:]
	if err := os.WriteFile(telemtConfigPath, []byte(text), 0600); err != nil {
		return TelemtProxy{}, err
	}
	if err := systemctl("daemon-reload"); err != nil {
		return TelemtProxy{}, err
	}

	if systemctl("is-active", "--quiet", telemtServiceName) == nil {
		if err := systemctl("restart", telemtServiceName); err != nil {
			_ = os.WriteFile(telemtConfigPath, old, 0600)
			_ = systemctl("daemon-reload")
			_ = systemctl("restart", telemtServiceName)
			return TelemtProxy{}, fmt.Errorf("telemt: proxy configuration was rejected: %w", err)
		}
	} else {
		if err := systemctl("start", telemtServiceName); err != nil {
			_ = os.WriteFile(telemtConfigPath, old, 0600)
			_ = systemctl("daemon-reload")
			return TelemtProxy{}, fmt.Errorf("telemt: failed to start service: %w", err)
		}
	}

	cfg, err := TelemtService{}.GetConfig()
	if err != nil {
		return TelemtProxy{}, err
	}

	link, err := telemtGeneratedLink(username, cfg.TLS)
	if err != nil {
		return TelemtProxy{}, fmt.Errorf("telemt: failed to obtain generated link: %w", err)
	}
	if u, err := url.Parse(link); err == nil && u.Query().Get("server") != "" {
		q := u.Query()
		q.Set("server", host)
		u.RawQuery = q.Encode()
		link = u.String()
	}

	return TelemtProxy{Name: name, Secret: secret, Host: host, Port: cfg.Port, TLS: cfg.TLS, Link: link}, nil
}

func (TelemtService) ListProxies() ([]TelemtProxy, error) {
	if err := ensureTelemtConfig(); err != nil { return nil, err }
	b, err := os.ReadFile(telemtConfigPath)
	if err != nil { return nil, err }
	var raw struct {
		Server struct { Port int `toml:"port"` } `toml:"server"`
		Censorship struct { TLSDomain string `toml:"tls_domain"` } `toml:"censorship"`
		Access struct { Users map[string]string `toml:"users"` } `toml:"access"`
		General struct { Modes struct { TLS bool `toml:"tls"` } `toml:"modes"` } `toml:"general"`
	}
	if err := toml.Unmarshal(b, &raw); err != nil { return nil, fmt.Errorf("telemt: parse config: %w", err) }
	out := make([]TelemtProxy, 0, len(raw.Access.Users))
	for username, secret := range raw.Access.Users {
		if username == "xui" {
			continue
		}
		link, linkErr := telemtGeneratedLink(username, raw.General.Modes.TLS)
		if linkErr != nil { continue }
		host := ""
		if u, err := url.Parse(link); err == nil {
			host = u.Query().Get("server")
		}
		if host == "" { host = "—" }
		out = append(out, TelemtProxy{Name: username, Secret: secret, Host: host, Port: raw.Server.Port, TLS: raw.General.Modes.TLS, Link: link})
	}

	// A Telemt user must be represented only once in the panel. This also
	// protects the UI from duplicate entries if an old config contains
	// repeated/generated users that resolve to the same client link.
	seen := make(map[string]struct{}, len(out))
	unique := out[:0]
	for _, item := range out {
		key := item.Link
		if key == "" {
			key = item.Name
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, item)
	}
	return unique, nil
}

func (TelemtService) DeleteProxy(username string) error {
	username = strings.TrimSpace(username)
	if username == "" || username == "xui" || strings.ContainsAny(username, "/\\\r\n") {
		return errors.New("telemt: invalid proxy username")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	endpoint := "http://127.0.0.1:9091/v1/users/" + url.PathEscape(username)
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("telemt: create delete request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telemt: delete proxy: %w", err)
	}
	defer resp.Body.Close()
	var envelope map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("telemt: decode delete response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if data, ok := envelope["error"].(map[string]interface{}); ok {
			if msg, ok := data["message"].(string); ok && msg != "" {
				return fmt.Errorf("telemt: delete proxy: %s", msg)
			}
		}
		return fmt.Errorf("telemt: delete proxy: HTTP %d", resp.StatusCode)
	}
	if ok, exists := envelope["ok"].(bool); exists && !ok {
		return errors.New("telemt: delete proxy rejected by API")
	}
	return nil
}

func telemtGeneratedLink(username string, tls bool) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	endpoint := "http://127.0.0.1:9091/v1/users/" + url.PathEscape(username)
	var lastErr error
	for i := 0; i < 10; i++ {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}
		var envelope map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&envelope)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}
		ok, _ := envelope["ok"].(bool)
		if !ok {
			lastErr = errors.New("Telemt API returned an error")
			time.Sleep(300 * time.Millisecond)
			continue
		}
		data, _ := envelope["data"].(map[string]interface{})
		linksObj, _ := data["links"].(map[string]interface{})
		key := "classic"
		if tls {
			key = "tls"
		} else if _, ok := linksObj["secure"]; ok {
			key = "secure"
		}
		links, _ := linksObj[key].([]interface{})
		if len(links) > 0 {
			if link, ok := links[0].(string); ok && link != "" {
				return link, nil
			}
		}
		lastErr = errors.New("Telemt API returned no client link")
		time.Sleep(300 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("Telemt API is unavailable")
	}
	return "", lastErr
}

func (TelemtService) Apply(action string) error {
	switch action {
	case "start", "stop", "restart", "enable", "disable":
		return systemctl(action, telemtServiceName)
	case "update":
		return telemtUpdate()
	case "meko-enable":
		if err := ensureTelemtMekoFixInstalled(); err != nil {
			return err
		}
		if err := systemctl("enable", "--now", telemtMekoServiceName); err != nil {
			return fmt.Errorf("telemt meko: enable failed: %w", err)
		}
		return nil
	case "meko-disable":
		_ = systemctl("disable", "--now", telemtMekoServiceName)
		return nil
	default:
		return errors.New("telemt: unsupported action")
	}
}

func ensureTelemtMekoFixInstalled() error {
	const scriptURL = "https://raw.githubusercontent.com/SawaMEN/3x-ui/main/telemt-meko-fix.sh"
	const unitURL = "https://raw.githubusercontent.com/SawaMEN/3x-ui/main/telemt-meko-fix.service"
	client := &http.Client{Timeout: 15 * time.Second}
	download := func(url, path string, mode os.FileMode) error {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil { return err }
		resp, err := client.Do(req)
		if err != nil { return fmt.Errorf("download %s: %w", url, err) }
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK { return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode) }
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil { return err }
		if len(data) == 0 { return errors.New("downloaded file is empty") }
		if err := os.WriteFile(path, data, mode); err != nil { return err }
		return os.Chmod(path, mode)
	}
	if _, err := os.Stat("/usr/local/x-ui/telemt-meko-fix.sh"); errors.Is(err, os.ErrNotExist) {
		if err := download(scriptURL, "/usr/local/x-ui/telemt-meko-fix.sh", 0700); err != nil {
			return fmt.Errorf("install MEKO fix script: %w", err)
		}
	}
	if _, err := os.Stat("/etc/systemd/system/telemt-meko-fix.service"); errors.Is(err, os.ErrNotExist) {
		if err := download(unitURL, "/etc/systemd/system/telemt-meko-fix.service", 0644); err != nil {
			return fmt.Errorf("install MEKO fix service: %w", err)
		}
	}
	if err := systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("telemt meko: daemon-reload failed: %w", err)
	}
	return nil
}

func systemctl(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "systemctl", args...).Run()
}
