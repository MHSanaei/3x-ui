// Package trusttunnel manages TrustTunnel endpoint processes alongside Xray.
// TrustTunnel is a separate VPN protocol binary that runs independently of xray-core.
package trusttunnel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

const (
	DefaultInstallPath = "/opt/trusttunnel"
	BinaryName         = "trusttunnel_endpoint"
)

// Settings holds the TrustTunnel configuration parsed from the inbound's Settings JSON.
type Settings struct {
	Hostname            string   `json:"hostname"`
	CertFile            string   `json:"certFile"`
	KeyFile             string   `json:"keyFile"`
	EnableHTTP1         bool     `json:"enableHttp1"`
	EnableHTTP2         bool     `json:"enableHttp2"`
	EnableQUIC          bool     `json:"enableQuic"`
	IPv6Available       bool     `json:"ipv6Available"`
	AllowPrivateNetwork bool     `json:"allowPrivateNetwork"`
	Clients             []Client `json:"clients"`
}

// Client represents a TrustTunnel user credential.
type Client struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Enable     bool   `json:"enable"`
	LimitIP    int    `json:"limitIp"`
	TotalGB    int64  `json:"totalGB"`
	ExpiryTime int64  `json:"expiryTime"`
	TgID       int64  `json:"tgId"`
	SubID      string `json:"subId"`
	Comment    string `json:"comment"`
	Reset      int    `json:"reset"`
	CreatedAt  int64  `json:"created_at,omitempty"`
	UpdatedAt  int64  `json:"updated_at,omitempty"`
}

// GetInstallPath returns the TrustTunnel binary installation directory.
func GetInstallPath() string {
	p := os.Getenv("TRUSTTUNNEL_PATH")
	if p != "" {
		return p
	}
	return DefaultInstallPath
}

// GetBinaryPath returns the full path to the TrustTunnel endpoint binary.
func GetBinaryPath() string {
	return filepath.Join(GetInstallPath(), BinaryName)
}

// GetConfigDir returns the directory for TrustTunnel config files for a given inbound tag.
func GetConfigDir(tag string) string {
	return filepath.Join(config.GetBinFolderPath(), "trusttunnel", tag)
}

// IsBinaryInstalled checks if the TrustTunnel binary exists.
func IsBinaryInstalled() bool {
	_, err := os.Stat(GetBinaryPath())
	return err == nil
}

// GetVersion returns the TrustTunnel binary version string.
func GetVersion() string {
	cmd := exec.Command(GetBinaryPath(), "--version")
	data, err := cmd.Output()
	if err != nil {
		return "Unknown"
	}
	return strings.TrimSpace(string(data))
}

// Process manages a single TrustTunnel endpoint process.
type Process struct {
	cmd       *exec.Cmd
	tag       string
	configDir string
	logWriter *logWriter
	exitErr   error
	startTime time.Time
}

type logWriter struct {
	buf      bytes.Buffer
	lastLine string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	n, err = w.buf.Write(p)
	lines := strings.Split(strings.TrimSpace(w.buf.String()), "\n")
	if len(lines) > 0 {
		w.lastLine = lines[len(lines)-1]
	}
	return
}

// NewProcess creates a new TrustTunnel process for the given inbound tag.
func NewProcess(tag string) *Process {
	return &Process{
		tag:       tag,
		configDir: GetConfigDir(tag),
		logWriter: &logWriter{},
		startTime: time.Now(),
	}
}

func (p *Process) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	return p.cmd.ProcessState == nil
}

func (p *Process) GetErr() error {
	return p.exitErr
}

func (p *Process) GetResult() string {
	if p.logWriter.lastLine == "" && p.exitErr != nil {
		return p.exitErr.Error()
	}
	return p.logWriter.lastLine
}

func (p *Process) GetUptime() uint64 {
	return uint64(time.Since(p.startTime).Seconds())
}

// WriteConfig generates TOML configuration files from an inbound's settings.
func (p *Process) WriteConfig(listen string, port int, settings Settings) error {
	if err := os.MkdirAll(p.configDir, 0o750); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	listenAddr := listen
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}
	listenAddr = fmt.Sprintf("%s:%d", listenAddr, port)

	// vpn.toml
	vpnToml := fmt.Sprintf(`listen_address = "%s"
ipv6_available = %t
allow_private_network_connections = %t
credentials_file = "credentials.toml"
`, listenAddr, settings.IPv6Available, settings.AllowPrivateNetwork)

	// Add protocol sections based on user selection
	if settings.EnableHTTP1 {
		vpnToml += "\n[listen_protocols.http1]\n"
	}
	if settings.EnableHTTP2 {
		vpnToml += "\n[listen_protocols.http2]\n"
	}
	if settings.EnableQUIC {
		vpnToml += "\n[listen_protocols.quic]\n"
	}

	if err := os.WriteFile(filepath.Join(p.configDir, "vpn.toml"), []byte(vpnToml), 0o640); err != nil {
		return fmt.Errorf("failed to write vpn.toml: %w", err)
	}

	// hosts.toml
	if settings.Hostname != "" && settings.CertFile != "" && settings.KeyFile != "" {
		hostsToml := fmt.Sprintf(`[[main_hosts]]
hostname = "%s"
cert_chain_path = "%s"
private_key_path = "%s"
`, settings.Hostname, settings.CertFile, settings.KeyFile)

		if err := os.WriteFile(filepath.Join(p.configDir, "hosts.toml"), []byte(hostsToml), 0o640); err != nil {
			return fmt.Errorf("failed to write hosts.toml: %w", err)
		}
	}

	// credentials.toml
	var credBuf strings.Builder
	for _, client := range settings.Clients {
		if !client.Enable {
			continue
		}
		credBuf.WriteString(fmt.Sprintf("[[client]]\nusername = \"%s\"\npassword = \"%s\"\n\n",
			escapeToml(client.Email), escapeToml(client.Password)))
	}
	if err := os.WriteFile(filepath.Join(p.configDir, "credentials.toml"), []byte(credBuf.String()), 0o640); err != nil {
		return fmt.Errorf("failed to write credentials.toml: %w", err)
	}

	return nil
}

// Start launches the TrustTunnel endpoint process.
// The binary expects positional args: trusttunnel_endpoint vpn.toml hosts.toml
func (p *Process) Start() error {
	if p.IsRunning() {
		return fmt.Errorf("trusttunnel %s is already running", p.tag)
	}

	if !IsBinaryInstalled() {
		return fmt.Errorf("trusttunnel binary not found at %s", GetBinaryPath())
	}

	cmd := exec.Command(GetBinaryPath(), "vpn.toml", "hosts.toml")
	cmd.Dir = p.configDir
	cmd.Stdout = p.logWriter
	cmd.Stderr = p.logWriter
	p.cmd = cmd
	p.startTime = time.Now()
	p.exitErr = nil

	go func() {
		err := cmd.Run()
		if err != nil {
			logger.Errorf("TrustTunnel process %s exited: %v", p.tag, err)
			p.exitErr = err
		}
	}()

	return nil
}

// Stop terminates the TrustTunnel endpoint process.
func (p *Process) Stop() error {
	if !p.IsRunning() {
		return nil
	}
	if runtime.GOOS == "windows" {
		return p.cmd.Process.Kill()
	}
	return p.cmd.Process.Signal(syscall.SIGTERM)
}

// ParseSettings parses TrustTunnel settings from the inbound's Settings JSON string.
func ParseSettings(settingsJSON string) (Settings, error) {
	var s Settings
	s.EnableHTTP1 = true
	s.EnableHTTP2 = true
	s.EnableQUIC = true
	s.IPv6Available = true
	if err := json.Unmarshal([]byte(settingsJSON), &s); err != nil {
		return s, fmt.Errorf("failed to parse trusttunnel settings: %w", err)
	}
	return s, nil
}

func escapeToml(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
