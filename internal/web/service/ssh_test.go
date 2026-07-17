package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

func TestNormalizeSSHEncryptsCredentials(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	svc := NodeService{}
	n := &model.Node{
		Name:        "ssh-1",
		Mode:        "ssh",
		Address:     "203.0.113.10",
		SshUser:     "root",
		SshAuthType: "password",
		SshPassword: "hunter2",
	}
	if err := svc.normalize(n); err != nil {
		t.Fatalf("normalize ssh node: %v", err)
	}
	if n.SshPassword == "hunter2" {
		t.Fatalf("SshPassword stored in plaintext")
	}
	if !crypto.IsEncrypted(n.SshPassword) {
		t.Fatalf("SshPassword = %q, want an encrypted value", n.SshPassword)
	}
	got, err := crypto.DecryptSecret(n.SshPassword)
	if err != nil {
		t.Fatalf("decrypt stored password: %v", err)
	}
	if got != "hunter2" {
		t.Fatalf("decrypted password = %q, want %q", got, "hunter2")
	}
	if n.SshPort != 22 {
		t.Fatalf("SshPort = %d, want default 22", n.SshPort)
	}
	if n.SshHostKeyMode != "trust" {
		t.Fatalf("SshHostKeyMode = %q, want default trust", n.SshHostKeyMode)
	}
}

func TestNormalizeSSHClearsApiFields(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	svc := NodeService{}
	n := &model.Node{
		Name:        "ssh-2",
		Mode:        "ssh",
		Address:     "203.0.113.11",
		SshUser:     "root",
		SshAuthType: "password",
		SshPassword: "pw",
		Port:        2053,
		ApiToken:    "leftover-token",
		Scheme:      "https",
		BasePath:    "/panel",
	}
	if err := svc.normalize(n); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if n.Port != 0 || n.ApiToken != "" || n.Scheme != "" || n.BasePath != "" {
		t.Fatalf("api fields not cleared: port=%d token=%q scheme=%q basePath=%q",
			n.Port, n.ApiToken, n.Scheme, n.BasePath)
	}
}

func TestNormalizeSSHRequiresCredential(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	svc := NodeService{}
	tests := []struct {
		name string
		node *model.Node
		want string
	}{
		{
			name: "password mode without password",
			node: &model.Node{Name: "x", Mode: "ssh", Address: "203.0.113.1", SshUser: "root", SshAuthType: "password"},
			want: "ssh password is required",
		},
		{
			name: "key mode without key",
			node: &model.Node{Name: "x", Mode: "ssh", Address: "203.0.113.1", SshUser: "root", SshAuthType: "key"},
			want: "ssh private key is required",
		},
		{
			name: "no username",
			node: &model.Node{Name: "x", Mode: "ssh", Address: "203.0.113.1", SshAuthType: "password", SshPassword: "pw"},
			want: "ssh username is required",
		},
		{
			name: "pin without fingerprint",
			node: &model.Node{Name: "x", Mode: "ssh", Address: "203.0.113.1", SshUser: "root", SshAuthType: "password", SshPassword: "pw", SshHostKeyMode: "pin"},
			want: "host key pinning requires a fingerprint",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.normalize(tt.node)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("normalize error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestNormalizeApiModeUnaffected(t *testing.T) {
	svc := NodeService{}
	n := &model.Node{Name: "api-1", Address: "node.example.com", Port: 2053, ApiToken: "tok"}
	if err := svc.normalize(n); err != nil {
		t.Fatalf("normalize api node: %v", err)
	}
	if n.Mode != "api" {
		t.Fatalf("Mode = %q, want default api", n.Mode)
	}
	if n.Scheme != "https" {
		t.Fatalf("Scheme = %q, want https", n.Scheme)
	}
}

func TestNormalizeApiModeStillRequiresToken(t *testing.T) {
	svc := NodeService{}
	n := &model.Node{Name: "api-2", Address: "node.example.com", Port: 2053, TlsVerifyMode: "verify"}
	err := svc.normalize(n)
	if err == nil || !strings.Contains(err.Error(), "api token is required") {
		t.Fatalf("normalize error = %v, want it to require an api token", err)
	}
}

func TestUpdateCarriesForwardSSHSecret(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	setupConflictDB(t)
	svc := NodeService{}

	n := &model.Node{
		Name:        "ssh-carry",
		Mode:        "ssh",
		Address:     "203.0.113.20",
		SshUser:     "root",
		SshAuthType: "password",
		SshPassword: "original-pw",
	}
	if err := svc.Create(n); err != nil {
		t.Fatalf("create: %v", err)
	}
	stored, err := svc.GetById(n.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	cipherBefore := stored.SshPassword

	edit := &model.Node{
		Name:        "ssh-carry-renamed",
		Mode:        "ssh",
		Address:     "203.0.113.20",
		SshUser:     "root",
		SshAuthType: "password",
	}
	if err := svc.Update(n.Id, edit); err != nil {
		t.Fatalf("update without re-entering password: %v", err)
	}
	after, err := svc.GetById(n.Id)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if after.Name != "ssh-carry-renamed" {
		t.Fatalf("rename did not apply: name=%q", after.Name)
	}
	if after.SshPassword != cipherBefore {
		t.Fatalf("stored password changed on an edit that did not re-enter it")
	}
	if !after.SshPasswordSet {
		t.Fatalf("SshPasswordSet = false, want true")
	}
	pw, err := crypto.DecryptSecret(after.SshPassword)
	if err != nil || pw != "original-pw" {
		t.Fatalf("decrypted carried-forward password = (%q, %v), want original-pw", pw, err)
	}
}

func TestSSHDialRejectsPrivateAddressWithoutOptIn(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	tests := []struct {
		name    string
		address string
	}{
		{name: "loopback", address: "127.0.0.1"},
		{name: "rfc1918", address: "10.0.0.5"},
		{name: "link local", address: "169.254.1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &model.Node{
				Mode:        "ssh",
				Address:     tt.address,
				SshPort:     22,
				SshUser:     "root",
				SshAuthType: "password",
				SshPassword: "pw",
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := (&SSHService{}).Dial(ctx, n)
			if err == nil {
				t.Fatalf("Dial to private %s succeeded without AllowPrivateAddress, want it blocked", tt.address)
			}
			if !strings.Contains(err.Error(), "blocked private/internal address") {
				t.Fatalf("Dial error = %v, want a blocked-private-address error", err)
			}
		})
	}
}

func TestUpdatePreservesTrustFingerprint(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	setupConflictDB(t)
	svc := NodeService{}

	n := &model.Node{
		Mode:        "ssh",
		Name:        "tofu-node",
		Address:     "203.0.113.30",
		SshUser:     "root",
		SshAuthType: "password",
		SshPassword: "pw",
	}
	if err := svc.Create(n); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Simulate the heartbeat learning the host key under trust-on-first-use.
	const learned = "sha256:learnedfingerprintvalue"
	if err := database.GetDB().Model(&model.Node{}).Where("id = ?", n.Id).
		Update("ssh_host_key_sha256", learned).Error; err != nil {
		t.Fatalf("seed fingerprint: %v", err)
	}

	// A plain rename that does not re-enter the fingerprint must keep the
	// learned anchor, not reset TOFU.
	edit := &model.Node{
		Mode:        "ssh",
		Name:        "tofu-node-renamed",
		Address:     "203.0.113.30",
		SshUser:     "root",
		SshAuthType: "password",
	}
	if err := svc.Update(n.Id, edit); err != nil {
		t.Fatalf("update: %v", err)
	}
	after, err := svc.GetById(n.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if after.SshHostKeySha256 != learned {
		t.Fatalf("SshHostKeySha256 = %q after edit, want the learned anchor %q preserved", after.SshHostKeySha256, learned)
	}
}

func TestProbeSSHUnreachable(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	svc := NodeService{}
	n := &model.Node{
		Mode:        "ssh",
		Address:     "203.0.113.255",
		SshPort:     1,
		SshUser:     "root",
		SshAuthType: "password",
		SshPassword: "pw",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	patch := svc.ProbeSSH(ctx, n)
	if patch.Status != "unreachable" {
		t.Fatalf("Status = %q, want unreachable", patch.Status)
	}
	if patch.LastError == "" {
		t.Fatalf("LastError is empty, want a connection error")
	}
}
