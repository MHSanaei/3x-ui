package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestCommand(t *testing.T, req CleanIpCommandRequest) *CleanIpCommand {
	t.Helper()
	var s CleanIpService
	cmd, err := s.NewCommand(req, "https://panel.example.com/secret/")
	if err != nil {
		t.Fatalf("NewCommand: %v", err)
	}
	return cmd
}

// A placeholder the panel forgets to fill is a Python NameError on the admin's device.
func TestCleanIpScriptFillsEveryPlaceholder(t *testing.T) {
	cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com", Path: "/ws", Ips: []string{"104.16.1.1"}})
	var s CleanIpService
	script, err := s.Script(cmd.Token, "https://panel.example.com/secret/cleanip/run/"+cmd.Token+"/report", "")
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	if left := regexp.MustCompile(`__[A-Z_]+__`).FindAllString(script, -1); len(left) > 0 {
		t.Fatalf("unfilled placeholders: %v", left)
	}
	for _, want := range []string{`HOST = "cdn.example.com"`, `PATH = "/ws"`, `FRAG_IPS = ["104.16.1.1"]`, `SIZE = 5000`} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %q", want)
		}
	}
	if !strings.HasPrefix(cmd.ScriptUrl, "https://panel.example.com/secret/cleanip/run/") ||
		cmd.Unix != "curl -fsSL '"+cmd.ScriptUrl+"' | python3 -" {
		t.Errorf("command = %q / %q", cmd.ScriptUrl, cmd.Unix)
	}
}

// The script's values end up as Python literals: a host with a quote would break out of one.
func TestCleanIpNewCommandRejectsUnsafeInput(t *testing.T) {
	var s CleanIpService
	for _, req := range []CleanIpCommandRequest{
		{Host: `cdn.example.com"); import os; ("`},
		{Host: "cdn.example.com", Path: "no-slash"},
		{Host: ""},
	} {
		if _, err := s.NewCommand(req, "https://p/"); !errors.Is(err, errCleanIpBadRequest) {
			t.Errorf("NewCommand(%+v) err = %v, want errCleanIpBadRequest", req, err)
		}
	}
}

// The report endpoint is reachable with the token alone, so its rows are untrusted input.
func TestCleanIpAddReportKeepsOnlyValidRows(t *testing.T) {
	cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com"})
	var s CleanIpService
	body := `{"tested": 120, "size": 5000,
	  "ips": [{"ip": "104.16.1.1", "ms": 410, "colo": "FRA"}, {"ip": "not-an-ip", "ms": 1}, {"ip": "::1", "ms": 2},
	          {"ip": "104.16.1.2", "ms": 99, "colo": "<script>"}],
	  "fragments": [{"packets": "tlshello", "length": "10-20", "delay": "1-2", "ok": 9, "n": 10, "ms": 700},
	                {"packets": "", "length": "", "delay": "", "ok": 2, "n": 10, "ms": 300},
	                {"packets": "rm -rf /", "length": "10-20", "delay": "1-2", "ok": 10, "n": 10},
	                {"packets": "1-1", "length": "abc", "delay": "1-2", "ok": 10, "n": 10}]}`
	report, err := s.AddReport(context.Background(), cmd.Token, []byte(body), "127.0.0.1")
	if err != nil {
		t.Fatalf("AddReport: %v", err)
	}
	wantIps := []CleanIpHit{{Ip: "104.16.1.2", Ms: 99}, {Ip: "104.16.1.1", Ms: 410, Colo: "FRA"}}
	if !reflect.DeepEqual(report.Ips, wantIps) {
		t.Errorf("ips = %+v, want %+v", report.Ips, wantIps)
	}
	wantFrags := []CleanIpFragment{
		{Packets: "tlshello", Length: "10-20", Delay: "1-2", Ok: 9, N: 10, Ms: 700},
		{Ok: 2, N: 10, Ms: 300},
	}
	if !reflect.DeepEqual(report.Fragments, wantFrags) {
		t.Errorf("fragments = %+v, want %+v", report.Fragments, wantFrags)
	}
	reports, err := s.Reports(cmd.Token)
	if err != nil || len(reports) != 1 || reports[0].Tested != 120 {
		t.Fatalf("Reports = %+v, %v; want the one report", reports, err)
	}
}

func TestCleanIpExpiredCommandRefusesReports(t *testing.T) {
	cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com"})
	defer func(prev func() time.Time) { cleanIpNow = prev }(cleanIpNow)
	cleanIpNow = func() time.Time { return time.Now().Add(cleanIpCommandTTL + time.Minute) }

	var s CleanIpService
	if _, err := s.AddReport(context.Background(), cmd.Token, []byte(`{}`), "127.0.0.1"); !errors.Is(err, errCleanIpExpired) {
		t.Errorf("AddReport err = %v, want errCleanIpExpired", err)
	}
	if _, err := s.Script(cmd.Token, "https://p/r", ""); !errors.Is(err, errCleanIpExpired) {
		t.Errorf("Script err = %v, want errCleanIpExpired", err)
	}
}

// CF-Connecting-IP is a plain request header: it may only replace the address when the
// request really came from Cloudflare, or anyone could pick the ISP a scan is saved for.
func TestCleanIpVisitorIp(t *testing.T) {
	for _, tc := range []struct{ remote, header, want string }{
		{"162.158.10.20", "5.160.10.20", "5.160.10.20"},
		{"2400:cb00::1", "5.160.10.20", "5.160.10.20"},
		{"5.200.1.1", "8.8.8.8", "5.200.1.1"},
		{"162.158.10.20", "not-an-ip", "162.158.10.20"},
		{"162.158.10.20", "", "162.158.10.20"},
	} {
		if got := CleanIpVisitorIp(tc.remote, tc.header); got != tc.want {
			t.Errorf("CleanIpVisitorIp(%q, %q) = %q, want %q", tc.remote, tc.header, got, tc.want)
		}
	}
}

// Cloudflare answers the TLS handshake on every edge IP, but on an IP that may not serve the
// zone it replies 403 "error code: 1034" - such an IP must not pass, an origin's own 403 must.
func TestCleanIpClassifyResponse(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      string
	}{
		{"edge restricted", "HTTP/1.1 403 Forbidden\r\nContent-Type: text/plain; charset=UTF-8\r\nServer: cloudflare\r\n\r\nerror code: 1034", cleanIpRestricted},
		{"other cloudflare 1xxx", "HTTP/1.1 403 Forbidden\r\nServer: cloudflare\r\n\r\nerror code: 1003", cleanIpRestricted},
		{"origin redirect", "HTTP/1.1 302 Found\r\nServer: cloudflare\r\nLocation: https://github.com/\r\n\r\n", cleanIpOk},
		{"origin 403 page", "HTTP/1.1 403 Forbidden\r\nServer: cloudflare\r\nContent-Type: text/html\r\n\r\n<html>forbidden by the site</html>", cleanIpOk},
		{"origin down is still a good IP", "HTTP/1.1 521 Web Server Is Down\r\nServer: cloudflare\r\n\r\nerror code: 521", cleanIpOk},
		{"not http", "garbage", cleanIpUnreachable},
	} {
		if got := classifyCleanIpResponse([]byte(tc.raw)); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The page reads report.fragments / report.ips as arrays; a JSON null crashed it.
func TestCleanIpReportWithNothingValidSendsEmptyLists(t *testing.T) {
	cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com"})
	var s CleanIpService
	report, err := s.AddReport(context.Background(), cmd.Token, []byte(`{"tested": 5, "ips": [{"ip": "x"}]}`), "127.0.0.1")
	if err != nil {
		t.Fatalf("AddReport: %v", err)
	}
	raw, _ := json.Marshal(report)
	for _, want := range []string{`"ips":[]`, `"fragments":[]`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("report JSON %s lacks %s", raw, want)
		}
	}
}

func TestCleanIpCommandSamplesTheChosenCdn(t *testing.T) {
	var s CleanIpService
	script := func(cdn string) string {
		t.Helper()
		cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com", Cdn: cdn})
		out, err := s.Script(cmd.Token, "https://p/r", "")
		if err != nil {
			t.Fatalf("Script(%q): %v", cdn, err)
		}
		return out
	}
	if got := script("fastly"); !strings.Contains(got, `"151.101.0.0/16"`) || strings.Contains(got, `"104.16.0.0/13"`) {
		t.Error("a Fastly command must sample Fastly's ranges, not Cloudflare's")
	}
	if got := script(""); !strings.Contains(got, `"104.16.0.0/13"`) {
		t.Error("a command without a CDN must sample Cloudflare, as before")
	}
	if _, err := s.NewCommand(CleanIpCommandRequest{Host: "cdn.example.com", Cdn: "akamai"}, "https://p/"); !errors.Is(err, errCleanIpBadRequest) {
		t.Errorf("unknown CDN: err = %v, want errCleanIpBadRequest", err)
	}
}

// The quick scan's hits get the upload test too: that is the only way the page learns their datacenter.
func TestCleanIpCommandKeepsUpToAHundredPageIps(t *testing.T) {
	ips := make([]string, 150)
	for i := range ips {
		ips[i] = "104.16." + strconv.Itoa(i/250) + "." + strconv.Itoa(i%250+1)
	}
	cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com", Ips: ips})
	cleanIpJobsMu.Lock()
	got := cleanIpJobs[cmd.Token].req.Ips
	cleanIpJobsMu.Unlock()
	if len(got) != 100 || got[0] != ips[0] || got[99] != ips[99] {
		t.Fatalf("kept %d page IPs (first %v); want the first 100", len(got), got[:1])
	}
}

func pythonForTest(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if out, err := exec.Command(name, "-c", "import sys; print(sys.version_info[0])").Output(); err == nil && strings.TrimSpace(string(out)) == "3" {
			return name
		}
	}
	t.Skip("no Python 3 to run the clean IP script")
	return ""
}

// Runs the real script's helpers (without main) on the reply heads each CDN sends.
func TestCleanIpScriptReadsTheDatacenterOfEveryCdnAndTestsPageIpsFirst(t *testing.T) {
	py := pythonForTest(t)
	cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com", Cdn: "fastly", PerRange: 1, Ips: []string{"151.101.1.1", "151.101.2.2"}})
	var s CleanIpService
	script, err := s.Script(cmd.Token, "https://p/r", "")
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	path := filepath.Join(t.TempDir(), "clean.py")
	harness := strings.Replace(script, "if __name__ == \"__main__\":", "if False:", 1) + `
heads = [b"HTTP/1.1 200 OK\r\nCF-RAY: 8f1c2d3e4a5b6c7d-FRA\r\n",
         b"HTTP/1.1 200 OK\r\nX-Served-By: cache-iad-kiad7000-IAD, cache-ams21-AMS\r\n",
         b"HTTP/1.1 200 OK\r\nX-Amz-Cf-Pop: IST50-P2\r\n",
         b"HTTP/1.1 200 OK\r\nX-ID: lon1-up-gc30\r\n",
         b"HTTP/1.1 200 OK\r\nServer: nginx\r\n"]
print(",".join(colo_of(h) for h in heads))
ips = targets()
print(ips[:2] == FRAG_IPS and len(ips) == len(set(ips)) and len(ips) > 2)
`
	if err := os.WriteFile(path, []byte(harness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-I", path).CombinedOutput()
	if err != nil {
		t.Fatalf("python: %v\n%s", err, out)
	}
	if got := strings.Fields(string(out)); len(got) != 2 || got[0] != "FRA,AMS,IST,LON," || got[1] != "True" {
		t.Fatalf("script printed %q; want colos FRA,AMS,IST,LON, and the page IPs first", out)
	}
}

// A run where every IP fails must say why (timeouts vs certificate vs refused), or the
// admin has nothing to go on.
func TestCleanIpScriptCountsWhyIpsFail(t *testing.T) {
	py := pythonForTest(t)
	cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com"})
	var s CleanIpService
	script, err := s.Script(cmd.Token, "https://p/r", "")
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	path := filepath.Join(t.TempDir(), "clean.py")
	harness := strings.Replace(script, "if __name__ == \"__main__\":", "if False:", 1) + `
probe("127.0.0.1")
print(failure_summary())
`
	if err := os.WriteFile(path, []byte(harness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-I", path).CombinedOutput()
	if err != nil {
		t.Fatalf("python: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "refused 1" {
		t.Fatalf("failure_summary() = %q after one probe of a closed port; want \"refused 1\"", got)
	}
}

// On an ISP that blocks the panel's domain the plain command dies in curl (seen on MCI);
// the "via" command reaches the panel through the fastest clean IP instead.
func TestCleanIpCommandViaTheFastestCleanIp(t *testing.T) {
	var s CleanIpService
	cmd, err := s.NewCommand(CleanIpCommandRequest{Host: "cdn.example.com", Ips: []string{"bad", "104.16.1.1", "104.16.2.2"}}, "https://ui.example.com/secret/")
	if err != nil {
		t.Fatalf("NewCommand: %v", err)
	}
	wantUnix := "curl -fsSL --resolve 'ui.example.com:443:104.16.1.1' '" + cmd.ScriptUrl + "?via=104.16.1.1' | python3 -"
	wantWin := "curl.exe -fsSL --resolve 'ui.example.com:443:104.16.1.1' '" + cmd.ScriptUrl + "?via=104.16.1.1' | python -"
	if cmd.Via != "104.16.1.1" || cmd.UnixVia != wantUnix || cmd.WindowsVia != wantWin {
		t.Errorf("via = %q\nunix = %q\nwant %q\nwindows = %q\nwant %q", cmd.Via, cmd.UnixVia, wantUnix, cmd.WindowsVia, wantWin)
	}

	cmd, err = s.NewCommand(CleanIpCommandRequest{Host: "cdn.example.com", Ips: []string{"104.16.1.1"}}, "https://203.0.113.5:2053/secret/")
	if err != nil || cmd.UnixVia != "" || cmd.WindowsVia != "" {
		t.Errorf("panel opened by IP has no domain to pin: via commands %q / %q (err %v), want none", cmd.UnixVia, cmd.WindowsVia, err)
	}
	if cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com"}); cmd.UnixVia != "" {
		t.Errorf("no clean IP found yet: via command %q, want none", cmd.UnixVia)
	}
	cmd, _ = s.NewCommand(CleanIpCommandRequest{Host: "cdn.example.com", Ips: []string{"104.16.1.1"}}, "https://ui.example.com:8443/")
	if !strings.Contains(cmd.UnixVia, "--resolve 'ui.example.com:8443:104.16.1.1'") {
		t.Errorf("non-443 panel port lost: %q", cmd.UnixVia)
	}
}

// The script fetched through the IP must report through it too, and only to an IPv4 literal.
func TestCleanIpScriptReportsThroughThePinnedIp(t *testing.T) {
	py := pythonForTest(t)
	cmd := newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com"})
	var s CleanIpService
	if script, _ := s.Script(cmd.Token, "https://p/r", "1.2.3.4\"; import os"); !strings.Contains(script, `PANEL_VIA = ""`) {
		t.Fatal("a via that is not an IPv4 address must be dropped")
	}
	script, err := s.Script(cmd.Token, "http://panel.invalid:{PORT}/r", "127.0.0.1")
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	path := filepath.Join(t.TempDir(), "clean.py")
	harness := `import http.server, threading
srv = http.server.HTTPServer(("127.0.0.1", 0), http.server.BaseHTTPRequestHandler)
srv.RequestHandlerClass.do_POST = lambda self: (self.send_response(200), self.end_headers(), self.wfile.write(b'{"isp": "pinned"}'))
srv.RequestHandlerClass.log_message = lambda *a: None
threading.Thread(target=srv.serve_forever, daemon=True).start()
` + strings.Replace(strings.Replace(script, "{PORT}", `" + str(srv.server_port) + "`, 1), "if __name__ == \"__main__\":", "if False:", 1) + `
pin_panel()
print(send_report(b"{}"))
`
	if err := os.WriteFile(path, []byte(harness), 0o600); err != nil {
		t.Fatal(err)
	}
	run := exec.Command(py, "-I", path)
	// a system HTTP proxy on the test machine would take the request instead of the pin
	run.Env = append(os.Environ(), "NO_PROXY=*", "no_proxy=*")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("python: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "pinned" {
		t.Fatalf("report through 127.0.0.1 for panel.invalid returned %q; want the local server's \"pinned\"", got)
	}
}

func TestCleanIpCommandScansExactlyTheTickedRanges(t *testing.T) {
	var s CleanIpService
	rangesOf := func(req CleanIpCommandRequest) []string {
		t.Helper()
		cmd := newTestCommand(t, req)
		cleanIpJobsMu.Lock()
		defer cleanIpJobsMu.Unlock()
		return cleanIpJobs[cmd.Token].req.Ranges
	}
	got := rangesOf(CleanIpCommandRequest{Host: "cdn.example.com", Ranges: []string{
		"104.16.0.0/13", "not-a-range", "104.16.0.0/13", "10.0.0.0/33", "2606:4700::/32", "1.2.3.4/32", " 172.64.0.0/24 ",
	}})
	if want := []string{"104.16.0.0/13", "172.64.0.0/24"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ticked ranges kept = %v, want %v (IPv4 /8../30, deduplicated)", got, want)
	}

	// nothing ticked: the CDN's default groups, never an opt-in AS group
	optIn := ""
	for _, g := range cleanIpCdns[0].Groups {
		if !g.On {
			optIn = g.Ranges[0]
			break
		}
	}
	defaults := rangesOf(CleanIpCommandRequest{Host: "cdn.example.com"})
	if !slices.Contains(defaults, "104.16.0.0/13") || slices.Contains(defaults, optIn) {
		t.Errorf("default ranges must be the on groups only (has 104.16.0.0/13: %v, has opt-in %s: %v)",
			slices.Contains(defaults, "104.16.0.0/13"), optIn, slices.Contains(defaults, optIn))
	}
	script, err := s.Script(newTestCommand(t, CleanIpCommandRequest{Host: "cdn.example.com", Ranges: []string{"151.101.0.0/16"}}).Token, "https://p/r", "")
	if err != nil || !strings.Contains(script, `RANGES = ["151.101.0.0/16"]`) {
		t.Errorf("script must sample exactly the ticked ranges (err %v)", err)
	}
}
