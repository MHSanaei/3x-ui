package amneziawg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	dataDir           = "/opt/amnezia"
	containerName     = "amnezia-awg"
	panelImageName    = "amnezia-awg-panel"
	awgConfPath       = "/opt/amnezia/awg/awg0.conf"
	startScriptPath   = "/opt/amnezia/start.sh"
	dockerfileDir     = "/opt/amnezia/docker"
)

// Manager manages AmneziaWG Docker containers.
type Manager struct {
	mu sync.Mutex
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide AWG manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{}
	})
	return manager
}

func containerNameFor(inboundID int) string {
	return fmt.Sprintf("%s-%d", containerName, inboundID)
}

// EnsureImage ensures the custom panel Docker image is built.
func (m *Manager) EnsureImage() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureCustomImage()
}

func (m *Manager) ensureCustomImage() error {
	if err := exec.Command("docker", "image", "inspect", panelImageName).Run(); err == nil {
		logger.Debugf("amneziawg: custom image %s already exists", panelImageName)
		return nil
	}

	logger.Infof("amneziawg: building custom image %s", panelImageName)

	if err := os.MkdirAll(dockerfileDir, 0o755); err != nil {
		return fmt.Errorf("failed to create docker build dir: %w", err)
	}

	df := fmt.Sprintf(`FROM amneziavpn/amneziawg-go:latest
RUN apk add --no-cache bash dumb-init
RUN mkdir -p /opt/amnezia/awg
RUN printf '#!/bin/bash\ntail -f /dev/null\n' > %s && chmod a+x %s
ENTRYPOINT [ "dumb-init", "%s" ]
`, startScriptPath, startScriptPath, startScriptPath)

	if err := os.WriteFile(filepath.Join(dockerfileDir, "Dockerfile"), []byte(df), 0o644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	build := exec.Command("docker", "build", "--no-cache", "--pull", "-t", panelImageName, dockerfileDir)
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("failed to build custom image: %w", err)
	}

	logger.Infof("amneziawg: custom image %s built", panelImageName)
	return nil
}

// EnsureInbound starts or updates a Docker container for an AWG inbound.
// It returns the updated settings JSON (with real keys populated from the
// container), or the original settings if no keys were generated.
func (m *Manager) EnsureInbound(inboundID int, port int, settings string) (string, error) {
	logger.Infof("[awg-debug] EnsureInbound: starting inboundID=%d port=%d", inboundID, port)

	m.mu.Lock()
	defer m.mu.Unlock()

	name := containerNameFor(inboundID)

	var parsed SettingsInbound
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, fmt.Errorf("failed to parse AWG settings: %w", err)
	}
	if parsed.Server == nil {
		return settings, fmt.Errorf("AWG settings missing server config")
	}

	_ = m.stopAndRemoveContainer(name)

	if err := os.MkdirAll(filepath.Join(dataDir, fmt.Sprintf("awg-%d", inboundID)), 0o755); err != nil {
		return settings, fmt.Errorf("failed to create data directory: %w", err)
	}

	if err := m.ensureCustomImage(); err != nil {
		return settings, fmt.Errorf("failed to build custom image: %w", err)
	}

	_ = m.ensureNetwork()

	portStr := fmt.Sprintf("%d:%d/udp", port, parsed.Server.ServerPort)

	args := []string{
		"run", "-d",
		"--log-driver", "none",
		"--restart", "always",
		"--privileged",
		"--cap-add=NET_ADMIN",
		"--cap-add=SYS_MODULE",
		"-p", portStr,
		"--network", "amnezia-dns-net",
		"-v", "/lib/modules:/lib/modules",
		"--sysctl", "net.ipv4.conf.all.src_valid_mark=1",
		"--name", name,
		panelImageName,
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return settings, fmt.Errorf("failed to start AWG container: %w", err)
	}
	logger.Infof("[awg-debug] container %s started", name)

	// Generate keys inside the container when they are missing or not in the
	// padded base64 format that awg expects (standard WireGuard 32-byte keys
	// are 44 padded chars; unpadded are 43).
	needsKeys := parsed.Server.PrivateKey == "" || len(parsed.Server.PrivateKey) != 44
	if needsKeys {
		logger.Infof("[awg-debug] privateKey needs regeneration (len=%d, expected 44)", len(parsed.Server.PrivateKey))
		keyOut, err := exec.Command("docker", "exec", name, "awg", "genkey").CombinedOutput()
		if err != nil {
			_ = m.stopAndRemoveContainer(name)
			return settings, fmt.Errorf("failed to generate private key: %s: %w", string(keyOut), err)
		}
		privKey := strings.TrimSpace(string(keyOut))
		logger.Infof("[awg-debug] generated private key len=%d", len(privKey))

		pubCmd := exec.Command("docker", "exec", "-i", name, "sh", "-c",
			"cat > /tmp/privkey && awg pubkey < /tmp/privkey && rm /tmp/privkey")
		pubCmd.Stdin = strings.NewReader(privKey + "\n")
		pubOut, err := pubCmd.CombinedOutput()
		if err != nil {
			_ = m.stopAndRemoveContainer(name)
			return settings, fmt.Errorf("failed to derive public key: %s: %w", string(pubOut), err)
		}
		pubKey := strings.TrimSpace(string(pubOut))

		pskOut, err := exec.Command("docker", "exec", name, "awg", "genpsk").CombinedOutput()
		if err != nil {
			_ = m.stopAndRemoveContainer(name)
			return settings, fmt.Errorf("failed to generate PSK: %s: %w", string(pskOut), err)
		}
		psk := strings.TrimSpace(string(pskOut))

		parsed.Server.PrivateKey = privKey
		parsed.Server.PublicKey = pubKey
		parsed.Server.PSK = psk
		logger.Infof("[awg-debug] keys generated: pubKey=%s", pubKey)

		updated, err := json.MarshalIndent(parsed, "", "  ")
		if err != nil {
			_ = m.stopAndRemoveContainer(name)
			return settings, fmt.Errorf("failed to marshal updated settings: %w", err)
		}
		settings = string(updated)
	}

	confContent := buildServerConf(parsed.Server)
	// Append peers
	for _, client := range parsed.Clients {
		if !client.Enable || client.PublicKey == "" {
			continue
		}
		confContent += fmt.Sprintf("\n[Peer]\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = %s/32\n",
			client.PublicKey, client.PresharedKey, client.AssignedIP)
	}

	tmpConf, err := os.CreateTemp("", "awg-*.conf")
	if err != nil {
		return settings, fmt.Errorf("failed to create temp config: %w", err)
	}
	if _, err := tmpConf.WriteString(confContent); err != nil {
		tmpConf.Close()
		os.Remove(tmpConf.Name())
		return settings, fmt.Errorf("failed to write temp config: %w", err)
	}
	tmpConf.Close()
	defer os.Remove(tmpConf.Name())

	if err := exec.Command("docker", "cp", tmpConf.Name(), fmt.Sprintf("%s:%s", name, awgConfPath)).Run(); err != nil {
		_ = m.stopAndRemoveContainer(name)
		return settings, fmt.Errorf("failed to copy config: %w", err)
	}
	logger.Infof("[awg-debug] config written to container")

	// Copy the real startup script that brings up awg0 on container (re)start
	realStart := buildRealStartupScript(parsed.Server)
	tmpStart, err := os.CreateTemp("", "awg-start-*.sh")
	if err != nil {
		return settings, fmt.Errorf("failed to create temp startup script: %w", err)
	}
	if _, err := tmpStart.WriteString(realStart); err != nil {
		tmpStart.Close()
		os.Remove(tmpStart.Name())
		return settings, fmt.Errorf("failed to write temp startup script: %w", err)
	}
	tmpStart.Close()
	defer os.Remove(tmpStart.Name())

	if err := exec.Command("docker", "cp", tmpStart.Name(), fmt.Sprintf("%s:%s", name, startScriptPath)).Run(); err != nil {
		_ = m.stopAndRemoveContainer(name)
		return settings, fmt.Errorf("failed to copy startup script: %w", err)
	}

	if err := exec.Command("docker", "exec", "-d", name, "sh", "-c",
		"chmod +x "+startScriptPath+" && exec "+startScriptPath).Run(); err != nil {
		logger.Warningf("amneziawg: startup script execution failed in %s: %v", name, err)
	}
	logger.Infof("[awg-debug] startup script executed")

	logger.Infof("[awg-debug] EnsureInbound completed for inbound %d", inboundID)
	return settings, nil
}

// RemoveInbound stops and removes the Docker container for an inbound.
func (m *Manager) RemoveInbound(inboundID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopAndRemoveContainer(containerNameFor(inboundID))
}

// AddPeer adds a client as a peer to the AWG server.
func (m *Manager) AddPeer(inboundID int, clientPubKey, clientPSK, clientIP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := containerNameFor(inboundID)
	if !m.containerExists(name) {
		return fmt.Errorf("container %s does not exist", name)
	}
	if err := m.addPeerToContainer(name, clientPubKey, clientPSK, clientIP); err != nil {
		return err
	}
	return m.syncConfig(name)
}

// RemovePeer removes a client peer from the AWG server.
func (m *Manager) RemovePeer(inboundID int, clientPubKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := containerNameFor(inboundID)
	if !m.containerExists(name) {
		return nil
	}
	if err := m.removePeerFromContainer(name, clientPubKey); err != nil {
		return err
	}
	return m.syncConfig(name)
}

// IsInboundRunning checks if the Docker container for an inbound is running.
func (m *Manager) IsInboundRunning(inboundID int) bool {
	name := containerNameFor(inboundID)
	out, err := exec.Command("docker", "ps", "--format", "{{.Names}}", "--filter", fmt.Sprintf("name=%s", name)).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), name)
}

// StopAll stops all AWG containers (called on panel shutdown).
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}", "--filter", "name="+containerName).CombinedOutput()
	if err != nil {
		return
	}
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		m.stopAndRemoveContainer(name)
	}
}

func (m *Manager) stopAndRemoveContainer(name string) error {
	exec.Command("docker", "stop", name).Run()
	exec.Command("docker", "rm", "-f", name).Run()
	_ = os.RemoveAll(filepath.Join(dataDir, strings.Replace(name, containerName+"-", "awg-", 1)))
	return nil
}

func (m *Manager) containerExists(name string) bool {
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}", "--filter", fmt.Sprintf("name=^/%s$", name)).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == name
}

func (m *Manager) ensureNetwork() error {
	out, err := exec.Command("docker", "network", "ls").CombinedOutput()
	if err != nil {
		return err
	}
	if strings.Contains(string(out), "amnezia-dns-net") {
		return nil
	}
	return exec.Command("docker", "network", "create",
		"--driver", "bridge",
		"--subnet=172.29.172.0/24",
		"--opt", "com.docker.network.bridge.name=amn0",
		"amnezia-dns-net",
	).Run()
}

func (m *Manager) addPeerToContainer(name, pubKey, psk, ip string) error {
	peerBlock := fmt.Sprintf("\n[Peer]\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = %s/32\n", pubKey, psk, ip)
	cmd := exec.Command("docker", "exec", "-i", name, "sh", "-c",
		fmt.Sprintf("cat >> %s", awgConfPath))
	cmd.Stdin = strings.NewReader(peerBlock)
	return cmd.Run()
}

func (m *Manager) removePeerFromContainer(name, pubKey string) error {
	script := fmt.Sprintf(`
awk -v pubkey='%s' '
  /^\[Interface\]/ { in_interface=1; print; next }
  /^\[Peer\]/ {
    if (in_interface) { in_interface=0 }
    if (in_peer && !found) print peer_block
    peer_block = $0; in_peer = 1; found = 0; next
  }
  in_interface { print; next }
  in_peer { peer_block = peer_block "\n" $0 }
  $0 ~ "PublicKey = " pubkey { found = 1 }
  END { if (in_peer && !found) print peer_block }
' %s > /tmp/awg0.conf.new && mv /tmp/awg0.conf.new %s
`, pubKey, awgConfPath, awgConfPath)
	return exec.Command("docker", "exec", name, "sh", "-c", script).Run()
}

func (m *Manager) syncConfig(name string) error {
	return exec.Command("docker", "exec", name, "sh", "-c",
		`if ip link show awg0 >/dev/null 2>&1; then awg syncconf awg0 <(awg-quick strip `+awgConfPath+`); else awg-quick up `+awgConfPath+`; fi`).Run()
}

func serverAddr(subnetIP string, cidr int) string {
	if strings.HasSuffix(subnetIP, ".0") {
		return strings.TrimSuffix(subnetIP, "0") + "1/" + fmt.Sprint(cidr)
	}
	return fmt.Sprintf("%s/%d", subnetIP, cidr)
}

func buildServerConf(s *ServerConfig) string {
	var b bytes.Buffer
	b.WriteString("[Interface]\n")
	b.WriteString(fmt.Sprintf("PrivateKey = %s\n", s.PrivateKey))
	b.WriteString(fmt.Sprintf("Address = %s\n", serverAddr(s.SubnetIP, s.SubnetCIDR)))
	b.WriteString(fmt.Sprintf("ListenPort = %d\n", s.ServerPort))
	b.WriteString(fmt.Sprintf("Jc = %d\n", s.Jc))
	b.WriteString(fmt.Sprintf("Jmin = %d\n", s.Jmin))
	b.WriteString(fmt.Sprintf("Jmax = %d\n", s.Jmax))
	b.WriteString(fmt.Sprintf("S1 = %d\n", s.S1))
	b.WriteString(fmt.Sprintf("S2 = %d\n", s.S2))
	b.WriteString(fmt.Sprintf("S3 = %d\n", s.S3))
	b.WriteString(fmt.Sprintf("S4 = %d\n", s.S4))
	b.WriteString(fmt.Sprintf("H1 = %s\n", s.H1))
	b.WriteString(fmt.Sprintf("H2 = %s\n", s.H2))
	b.WriteString(fmt.Sprintf("H3 = %s\n", s.H3))
	b.WriteString(fmt.Sprintf("H4 = %s\n", s.H4))
	return b.String()
}

// buildRealStartupScript returns the real startup script that brings up awg0
// and sets iptables rules. This is written into the container to replace the
// dummy start.sh so that on container restart the interface comes up automatically.
func buildRealStartupScript(s *ServerConfig) string {
	subnet := fmt.Sprintf("%s/%d", s.SubnetIP, s.SubnetCIDR)
	return fmt.Sprintf(`#!/bin/bash
awg-quick down /opt/amnezia/awg/awg0.conf 2>/dev/null
if [ -f /opt/amnezia/awg/awg0.conf ]; then
  awg-quick up /opt/amnezia/awg/awg0.conf
fi
iptables -A INPUT -i awg0 -j ACCEPT 2>/dev/null
iptables -A FORWARD -i awg0 -j ACCEPT 2>/dev/null
iptables -A OUTPUT -o awg0 -j ACCEPT 2>/dev/null
iptables -A FORWARD -i awg0 -o eth0 -s %s -j ACCEPT 2>/dev/null
iptables -A FORWARD -i awg0 -o eth1 -s %s -j ACCEPT 2>/dev/null
iptables -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT 2>/dev/null
iptables -t nat -A POSTROUTING -s %s -o eth0 -j MASQUERADE 2>/dev/null
iptables -t nat -A POSTROUTING -s %s -o eth1 -j MASQUERADE 2>/dev/null
tail -f /dev/null
`, subnet, subnet, subnet, subnet)
}

// BuildClientConfig generates a client configuration file content for download.
func BuildClientConfig(server *ServerConfig, client ClientSettings, serverIP string) string {
	var b bytes.Buffer
	b.WriteString("[Interface]\n")
	b.WriteString(fmt.Sprintf("Address = %s/32\n", client.AssignedIP))
	b.WriteString(fmt.Sprintf("DNS = %s, %s\n", server.PrimaryDNS, server.SecondaryDNS))
	b.WriteString(fmt.Sprintf("PrivateKey = %s\n", client.PrivateKey))
	b.WriteString(fmt.Sprintf("Jc = %d\n", server.Jc))
	b.WriteString(fmt.Sprintf("Jmin = %d\n", server.Jmin))
	b.WriteString(fmt.Sprintf("Jmax = %d\n", server.Jmax))
	b.WriteString(fmt.Sprintf("S1 = %d\n", server.S1))
	b.WriteString(fmt.Sprintf("S2 = %d\n", server.S2))
	b.WriteString(fmt.Sprintf("S3 = %d\n", server.S3))
	b.WriteString(fmt.Sprintf("S4 = %d\n", server.S4))
	b.WriteString(fmt.Sprintf("H1 = %s\n", server.H1))
	b.WriteString(fmt.Sprintf("H2 = %s\n", server.H2))
	b.WriteString(fmt.Sprintf("H3 = %s\n", server.H3))
	b.WriteString(fmt.Sprintf("H4 = %s\n", server.H4))
	b.WriteString("\n[Peer]\n")
	b.WriteString(fmt.Sprintf("PublicKey = %s\n", server.PublicKey))
	b.WriteString(fmt.Sprintf("PresharedKey = %s\n", client.PresharedKey))
	b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	b.WriteString(fmt.Sprintf("Endpoint = %s:%d\n", serverIP, server.ServerPort))
	b.WriteString("PersistentKeepalive = 25\n")
	return b.String()
}
