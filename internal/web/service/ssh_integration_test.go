package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"golang.org/x/crypto/ssh"
)

// startTestSSHServer stands up a real SSH server on localhost that accepts one
// password and answers `cat /etc/os-release` with a canned payload, so the SSH
// service can be exercised end to end without a remote host. It returns the
// listen host, port, and the server's host-key fingerprint.
func startTestSSHServer(t *testing.T, wantUser, wantPassword string) (string, int, string) {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if conn.User() == wantUser && string(pass) == wantPassword {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("permission denied")
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveOneSSHConn(conn, cfg)
		}
	}()

	fingerprint := FormatHostKeyFingerprint(signer.PublicKey())
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return host, port, fingerprint
}

func serveOneSSHConn(conn net.Conn, cfg *ssh.ServerConfig) {
	defer conn.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)
	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only session")
			continue
		}
		ch, chReqs, err := newChan.Accept()
		if err != nil {
			continue
		}
		go handleSSHSession(ch, chReqs)
	}
}

func handleSSHSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	for req := range reqs {
		switch req.Type {
		case "exec":
			var payload struct{ Command string }
			_ = ssh.Unmarshal(req.Payload, &payload)
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
			if strings.Contains(payload.Command, "os-release") {
				_, _ = io.WriteString(ch, "NAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\nID=ubuntu\n")
			}
			_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
			_ = ch.Close()
			return
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

func TestSSHServiceTestConnectionSuccess(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	host, port, fingerprint := startTestSSHServer(t, "root", "s3cret")

	n := &model.Node{
		Mode:                "ssh",
		Address:             host,
		SshPort:             port,
		SshUser:             "root",
		SshAuthType:         "password",
		SshPassword:         "s3cret",
		SshHostKeyMode:      "trust",
		AllowPrivateAddress: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res := (&SSHService{}).TestConnection(ctx, n)

	if !res.Success {
		t.Fatalf("TestConnection failed: %q", res.Message)
	}
	if res.HostKeySha256 != fingerprint {
		t.Fatalf("HostKeySha256 = %q, want %q", res.HostKeySha256, fingerprint)
	}
	if res.OsName != "Ubuntu" || res.OsVersion != "24.04" {
		t.Fatalf("OS = %q %q, want Ubuntu 24.04", res.OsName, res.OsVersion)
	}
}

func TestSSHServiceWrongPassword(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	host, port, _ := startTestSSHServer(t, "root", "s3cret")

	n := &model.Node{
		Mode:                "ssh",
		Address:             host,
		SshPort:             port,
		SshUser:             "root",
		SshAuthType:         "password",
		SshPassword:         "wrong",
		SshHostKeyMode:      "trust",
		AllowPrivateAddress: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res := (&SSHService{}).TestConnection(ctx, n)
	if res.Success {
		t.Fatal("TestConnection succeeded with a wrong password, want failure")
	}
}

func TestSSHServiceHostKeyPinMismatch(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	host, port, _ := startTestSSHServer(t, "root", "s3cret")

	n := &model.Node{
		Mode:                "ssh",
		Address:             host,
		SshPort:             port,
		SshUser:             "root",
		SshAuthType:         "password",
		SshPassword:         "s3cret",
		SshHostKeyMode:      "pin",
		SshHostKeySha256:    "sha256:deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		AllowPrivateAddress: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res := (&SSHService{}).TestConnection(ctx, n)
	if res.Success {
		t.Fatal("connection succeeded despite host-key mismatch, want failure")
	}
	if !strings.Contains(res.Message, "host key mismatch") {
		t.Fatalf("Message = %q, want a host key mismatch error", res.Message)
	}
}

func TestSSHServiceEncryptedCredentialDecrypts(t *testing.T) {
	t.Setenv("XUI_SECRET_KEY", "test-key")
	host, port, _ := startTestSSHServer(t, "root", "s3cret")

	svc := NodeService{}
	n := &model.Node{
		Mode:                "ssh",
		Name:                "enc-node",
		Address:             host,
		SshPort:             port,
		SshUser:             "root",
		SshAuthType:         "password",
		SshPassword:         "s3cret",
		SshHostKeyMode:      "trust",
		AllowPrivateAddress: true,
	}
	if err := svc.normalize(n); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res := (&SSHService{}).TestConnection(ctx, n)
	if !res.Success {
		t.Fatalf("connection with encrypted-at-rest password failed: %q", res.Message)
	}
}
