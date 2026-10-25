package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

func TestDnsTunnelProfilesRoundTripAndValidation(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	var s DnsTunnelService

	got, err := s.GetProfiles()
	if err != nil || len(got) != 0 {
		t.Fatalf("fresh panel: profiles = %+v, %v; want none", got, err)
	}
	want := []DnsTunnelProfile{
		{Name: "Iran", Engine: "stormdns", Domain: "st.t.example.com", Key: "93d3404c6701dc9dea2d1ff3282ab88b", Method: 1},
		{Name: "Iran dnstt", Engine: "dnstt", Domain: "t.example.com", Key: "54b79ccdbd08b54284286dd00e8296fc448b37f9aa6e973c319f12c3d356c359"},
	}
	if err := s.SaveProfiles(want); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}
	if got, err = s.GetProfiles(); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("GetProfiles = %+v, %v; want %+v", got, err, want)
	}

	// each of these would end up in a link or a script the admin hands to users
	for _, bad := range []DnsTunnelProfile{
		{Name: "x", Engine: "ssh", Domain: "t.example.com", Key: "abcdefgh12345678"},
		{Name: "x", Engine: "stormdns", Domain: "t.example.com\" evil", Key: "abcdefgh12345678"},
		{Name: "x", Engine: "stormdns", Domain: "t.example.com", Key: "short"},
		{Name: "x", Engine: "stormdns", Domain: "t.example.com", Key: "abcdefgh12345678", Method: 9},
	} {
		if err := s.SaveProfiles([]DnsTunnelProfile{bad}); !errors.Is(err, errDnsTunnelBadProfile) {
			t.Errorf("SaveProfiles(%+v) err = %v, want errDnsTunnelBadProfile", bad, err)
		}
	}
	if got, _ = s.GetProfiles(); !reflect.DeepEqual(got, want) {
		t.Errorf("a refused save changed the stored profiles: %+v", got)
	}
}

func TestDnsTunnelScriptFillsEveryPlaceholder(t *testing.T) {
	var s DnsTunnelService
	cmd, err := s.NewCommand(DnsTunnelCommandRequest{Zone: "t.example.com", Resolvers: []string{"178.22.122.100", "2001:4860:4860::8888", "bad ip"}}, "https://panel.example.com/p/")
	if err != nil {
		t.Fatalf("NewCommand: %v", err)
	}
	script, err := s.Script(cmd.Token, "https://panel.example.com/p/dnstunnel/run/"+cmd.Token+"/report")
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	if left := regexp.MustCompile(`__[A-Z_]+__`).FindAllString(script, -1); len(left) > 0 {
		t.Fatalf("unfilled placeholders: %v", left)
	}
	for _, want := range []string{`ZONE = "t.example.com"`, `RESOLVERS = ["178.22.122.100","2001:4860:4860::8888"]`} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %q", want)
		}
	}
	if _, err := s.NewCommand(DnsTunnelCommandRequest{Zone: "t.example.com'); import os; ('"}, "https://p/"); !errors.Is(err, errCleanIpBadRequest) {
		t.Errorf("unsafe zone accepted: %v", err)
	}
}

// The report URL works with the token alone: only well-formed rows may be kept.
func TestDnsTunnelReportKeepsOnlyValidRows(t *testing.T) {
	var s DnsTunnelService
	cmd, err := s.NewCommand(DnsTunnelCommandRequest{Zone: "t.example.com"}, "https://p/")
	if err != nil {
		t.Fatalf("NewCommand: %v", err)
	}
	body := `{"results": [
	  {"ip": "178.22.122.100", "status": "ok", "ms": 120, "name": "Shecan"},
	  {"ip": "8.8.8.8", "status": "timeout", "ms": 3000, "name": "Google"},
	  {"ip": "not-an-ip", "status": "ok", "ms": 1},
	  {"ip": "1.1.1.1", "status": "rm -rf", "ms": 5},
	  {"ip": "9.9.9.9", "status": "ok", "ms": 80, "name": "<b>x</b>"}]}`
	report, err := s.AddReport(context.Background(), cmd.Token, []byte(body), "127.0.0.1")
	if err != nil {
		t.Fatalf("AddReport: %v", err)
	}
	want := []DnsResolverResult{
		{Ip: "9.9.9.9", Status: "ok", Ms: 80},
		{Ip: "178.22.122.100", Status: "ok", Ms: 120, Name: "Shecan"},
		{Ip: "8.8.8.8", Status: "timeout", Ms: 3000, Name: "Google"},
	}
	if !reflect.DeepEqual(report.Results, want) {
		t.Errorf("results = %+v, want %+v", report.Results, want)
	}
}

func TestDetectStormDnsServerReadsItsConfigAndKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(t.TempDir(), "key.txt")
	writeFile(t, keyFile, "93d3404c6701dc9dea2d1ff3282ab88b\n")
	writeFile(t, filepath.Join(dir, "server_config.toml"), strings.Join([]string{
		`# StormDNS server`,
		`DOMAIN = ["st.t.example.com", "ST.t2.example.org."]  # both zones`,
		`UDP_PORT = 5355`,
		`DATA_ENCRYPTION_METHOD = 2`,
		`ENCRYPTION_KEY_FILE = "` + filepath.ToSlash(keyFile) + `"`,
	}, "\n"))
	var s DnsTunnelService

	got, err := s.DetectStormDns(dir)
	want := []DnsTunnelProfile{
		{Name: "StormDNS", Engine: "stormdns", Domain: "st.t.example.com", Key: "93d3404c6701dc9dea2d1ff3282ab88b", Method: 2},
		{Name: "StormDNS 2", Engine: "stormdns", Domain: "st.t2.example.org", Key: "93d3404c6701dc9dea2d1ff3282ab88b", Method: 2},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectStormDns = %+v, %v; want %+v", got, err, want)
	}

	// without ENCRYPTION_KEY_FILE the server keeps encrypt_key.txt next to its config
	writeFile(t, filepath.Join(dir, "server_config.toml"), "DOMAIN = ['st.t.example.com']\nDATA_ENCRYPTION_METHOD = 1\n")
	writeFile(t, filepath.Join(dir, "encrypt_key.txt"), "abcdefgh12345678abcdefgh12345678")
	got, err = s.DetectStormDns(dir)
	if err != nil || len(got) != 1 || got[0].Key != "abcdefgh12345678abcdefgh12345678" || got[0].Method != 1 {
		t.Fatalf("default key file: %+v, %v", got, err)
	}

	if _, err := s.DetectStormDns(t.TempDir()); !errors.Is(err, errDnsTunnelNoServer) {
		t.Fatalf("empty dir: err = %v; want errDnsTunnelNoServer", err)
	}
	if _, err := s.DetectStormDns("relative/dir"); !errors.Is(err, errDnsTunnelNoServer) {
		t.Fatalf("relative dir: err = %v; want errDnsTunnelNoServer", err)
	}
	writeFile(t, filepath.Join(dir, "encrypt_key.txt"), "short")
	if _, err := s.DetectStormDns(dir); !errors.Is(err, errDnsTunnelBadProfile) {
		t.Fatalf("bad key: err = %v; want errDnsTunnelBadProfile", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
