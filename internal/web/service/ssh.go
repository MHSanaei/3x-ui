package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"

	"golang.org/x/crypto/ssh"
)

const (
	sshDialTimeout    = 10 * time.Second
	sshCommandTimeout = 30 * time.Second
)

// SSHService opens SSH sessions to nodes running in "ssh" access mode: servers
// reachable over SSH that may not have a 3x-ui panel installed yet.
type SSHService struct{}

// SSHDialResult reports the outcome of a connection attempt. HostKeySha256 is
// the key the server actually presented, so a caller doing trust-on-first-use
// can persist it, and a caller testing a connection can show the operator the
// fingerprint they are about to trust.
type SSHDialResult struct {
	HostKeySha256 string
	Client        *ssh.Client
}

// FormatHostKeyFingerprint renders a host key in the sha256:BASE64 form that
// OpenSSH prints, so an operator can compare it against `ssh-keyscan` output.
func FormatHostKeyFingerprint(key ssh.PublicKey) string {
	sum := sha256.Sum256(key.Marshal())
	return "sha256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

func sshAuthMethods(n *model.Node) ([]ssh.AuthMethod, error) {
	switch n.SshAuthType {
	case "key":
		privateKey, err := crypto.DecryptSecret(n.SshPrivateKey)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(privateKey) == "" {
			return nil, fmt.Errorf("ssh private key is required")
		}
		passphrase, err := crypto.DecryptSecret(n.SshKeyPassphrase)
		if err != nil {
			return nil, err
		}
		var signer ssh.Signer
		if passphrase == "" {
			signer, err = ssh.ParsePrivateKey([]byte(privateKey))
		} else {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
		}
		if err != nil {
			var missing *ssh.PassphraseMissingError
			if errors.As(err, &missing) {
				return nil, fmt.Errorf("ssh private key is passphrase-protected; a passphrase is required")
			}
			return nil, fmt.Errorf("ssh private key could not be parsed")
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		password, err := crypto.DecryptSecret(n.SshPassword)
		if err != nil {
			return nil, err
		}
		if password == "" {
			return nil, fmt.Errorf("ssh password is required")
		}
		return []ssh.AuthMethod{ssh.Password(password)}, nil
	}
}

// hostKeyCallback enforces the node's SshHostKeyMode. "pin" verifies against the
// stored fingerprint and fails on mismatch; "trust" accepts any key on the first
// connect and pins it afterwards; "skip" accepts anything. Accepting an unknown
// host key means handing the credential to whoever answered on the port, so
// "trust" records what it saw and "pin" refuses to be silently re-pointed.
func hostKeyCallback(n *model.Node, seen *string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fingerprint := FormatHostKeyFingerprint(key)
		*seen = fingerprint
		switch n.SshHostKeyMode {
		case "skip":
			return nil
		case "pin":
			expected := strings.TrimSpace(n.SshHostKeySha256)
			if expected == "" {
				return fmt.Errorf("host key pinning is enabled but no fingerprint is stored for this node")
			}
			if !strings.EqualFold(expected, fingerprint) {
				return fmt.Errorf("host key mismatch: expected %s, server presented %s", expected, fingerprint)
			}
			return nil
		default:
			pinned := strings.TrimSpace(n.SshHostKeySha256)
			if pinned != "" && !strings.EqualFold(pinned, fingerprint) {
				return fmt.Errorf("host key changed: expected %s, server presented %s", pinned, fingerprint)
			}
			return nil
		}
	}
}

// Dial opens an SSH connection to the node. The caller owns the returned client
// and must Close it.
func (s *SSHService) Dial(ctx context.Context, n *model.Node) (*SSHDialResult, error) {
	host, err := netsafe.NormalizeHost(n.Address)
	if err != nil {
		return nil, err
	}
	port := n.SshPort
	if port <= 0 {
		port = 22
	}
	authMethods, err := sshAuthMethods(n)
	if err != nil {
		return nil, err
	}
	user := strings.TrimSpace(n.SshUser)
	if user == "" {
		return nil, fmt.Errorf("ssh username is required")
	}

	var seenHostKey string
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback(n, &seenHostKey),
		Timeout:         sshDialTimeout,
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	// Resolve and dial through the shared SSRF guard so a hostname cannot escape
	// the private-address protection: it checks every resolved IP against
	// IsBlockedIP unless AllowPrivateAddress is set, exactly as the HTTP node
	// probe does. A bare net.Dialer would trust whatever the OS resolver
	// returned and leak the SSH credential to an internal host.
	dialCtx := netsafe.ContextWithAllowPrivate(ctx, n.AllowPrivateAddress)
	conn, err := netsafe.SSRFGuardedDialContext(dialCtx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", addr, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return &SSHDialResult{HostKeySha256: seenHostKey}, err
	}
	_ = conn.SetDeadline(time.Time{})
	return &SSHDialResult{
		HostKeySha256: seenHostKey,
		Client:        ssh.NewClient(clientConn, chans, reqs),
	}, nil
}

// RunCommand executes cmd on the node and returns its combined output.
func (s *SSHService) RunCommand(ctx context.Context, n *model.Node, cmd string) (string, error) {
	res, err := s.Dial(ctx, n)
	if err != nil {
		return "", err
	}
	defer res.Client.Close()
	return runOnClient(ctx, res.Client, cmd)
}

func runOnClient(ctx context.Context, client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	type result struct {
		out []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, runErr := session.CombinedOutput(cmd)
		done <- result{out: out, err: runErr}
	}()
	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return "", ctx.Err()
	case r := <-done:
		return string(r.out), r.err
	}
}

// SSHTestResult is what the panel shows after a connection test.
type SSHTestResult struct {
	Success       bool   `json:"success" example:"true"`
	Message       string `json:"message,omitempty" example:"Authentication failed"`
	HostKeySha256 string `json:"hostKeySha256,omitempty" example:"sha256:abc123"`
	OsName        string `json:"osName,omitempty" example:"Ubuntu"`
	OsVersion     string `json:"osVersion,omitempty" example:"24.04"`
}

// TestConnection verifies the node's SSH credentials and reports the host key
// and detected OS. It never returns the credential itself, and its error text
// is the transport's, which does not echo the password or key.
func (s *SSHService) TestConnection(ctx context.Context, n *model.Node) *SSHTestResult {
	res, err := s.Dial(ctx, n)
	if err != nil {
		out := &SSHTestResult{Success: false, Message: err.Error()}
		if res != nil {
			out.HostKeySha256 = res.HostKeySha256
		}
		return out
	}
	defer res.Client.Close()

	out := &SSHTestResult{Success: true, HostKeySha256: res.HostKeySha256}
	// Bound the OS probe on its own timeout so a host that accepts the
	// connection but stalls on the command can't hold the test open for the
	// whole parent budget.
	osCtx, cancel := context.WithTimeout(ctx, sshCommandTimeout)
	defer cancel()
	if release, err := runOnClient(osCtx, res.Client, "cat /etc/os-release"); err == nil {
		name, version := parseOsRelease(release)
		out.OsName = name
		out.OsVersion = version
	}
	return out
}

// parseOsRelease pulls the distro name and version out of /etc/os-release,
// preferring the human-readable NAME/VERSION_ID pair.
func parseOsRelease(content string) (string, string) {
	fields := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		fields[key] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	name := fields["NAME"]
	if name == "" {
		name = fields["ID"]
	}
	version := fields["VERSION_ID"]
	if version == "" {
		version = fields["VERSION"]
	}
	return name, version
}
