package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func resetRealityFinder(t *testing.T, now time.Time) *time.Time {
	t.Helper()
	clock := now
	realityFinderJobsMu.Lock()
	realityFinderJobs = map[string]*realityFinderJob{}
	realityFinderJobsMu.Unlock()
	realityFinderNow = func() time.Time { return clock }
	t.Cleanup(func() { realityFinderNow = time.Now })
	return &clock
}

func newFinderCommand(t *testing.T, req RealityFinderCommandRequest) *RealityFinderCommand {
	t.Helper()
	cmd, err := (&RealityFinderService{}).NewCommand(req, "https://panel.example.com:2053/secret/")
	if err != nil {
		t.Fatalf("NewCommand(%+v): %v", req, err)
	}
	return cmd
}

func TestRealityFinderNewCommandRejectsWhatTheScriptCannotScan(t *testing.T) {
	resetRealityFinder(t, time.Unix(1_800_000_000, 0))
	for _, req := range []RealityFinderCommandRequest{
		{Server: "10.0.0.5", Port: 443},
		{Server: "192.168.1.1", Port: 443},
		{Server: "2001:db8::1", Port: 443},
		{Server: "203.0.113.10'); import os; ('", Port: 443},
		{Server: "203.0.113.10", Port: 70000},
		{Server: "203.0.113.10", Port: 443, Scope: "16"},
	} {
		_, err := (&RealityFinderService{}).NewCommand(req, "https://panel.example.com/")
		if !errors.Is(err, errRealityFinderBadRequest) {
			t.Errorf("NewCommand(%+v) = %v, want errRealityFinderBadRequest", req, err)
		}
	}
}

func TestRealityFinderScriptCarriesTheJob(t *testing.T) {
	resetRealityFinder(t, time.Unix(1_800_000_000, 0))
	cmd := newFinderCommand(t, RealityFinderCommandRequest{
		Server: "203.0.113.10", Port: 8443, Scope: "prefix", Current: "WWW.Example.com",
	})
	wantURL := "https://panel.example.com:2053/secret/realityfinder/run/" + cmd.Token + "/script.py"
	if cmd.ScriptUrl != wantURL || !strings.Contains(cmd.Unix, "'"+wantURL+"'") || !strings.HasPrefix(cmd.Windows, "python -c ") {
		t.Fatalf("command = %+v, want both lines to fetch %s", cmd, wantURL)
	}
	// Cloudflare refuses Python's default User-Agent (error 1010), so the fetch must name one.
	for _, line := range []string{cmd.Unix, cmd.Windows} {
		if !strings.Contains(line, "'User-Agent':'Mozilla/5.0") {
			t.Errorf("command %q fetches the script with Python's default User-Agent", line)
		}
	}
	script, err := (&RealityFinderService{}).Script(cmd.Token)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`SERVER = "203.0.113.10"`, `PORT = 8443`, `SCOPE = "prefix"`,
		`KNOWN = "www.example.com"`,
		`REPORT = "https://panel.example.com:2053/secret/realityfinder/run/` + cmd.Token + `/report"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %q", want)
		}
	}
	if strings.Contains(script, "__SERVER__") || strings.Contains(script, "__REPORT__") {
		t.Error("script still has a placeholder")
	}
}

func TestRealityFinderCommandExpiresAfterADay(t *testing.T) {
	clock := resetRealityFinder(t, time.Unix(1_800_000_000, 0))
	cmd := newFinderCommand(t, RealityFinderCommandRequest{Server: "203.0.113.10", Port: 443})
	*clock = clock.Add(realityFinderCommandTTL + time.Second)
	if _, err := (&RealityFinderService{}).Script(cmd.Token); !errors.Is(err, errRealityFinderExpired) {
		t.Errorf("Script after a day = %v, want errRealityFinderExpired", err)
	}
	if _, err := (&RealityFinderService{}).AddReport(cmd.Token, []byte(`{}`), "5.120.1.2"); !errors.Is(err, errRealityFinderExpired) {
		t.Errorf("AddReport after a day = %v, want errRealityFinderExpired", err)
	}
}

func TestRealityFinderAddReportKeepsOnlyCleanTargets(t *testing.T) {
	resetRealityFinder(t, time.Unix(1_800_000_000, 0))
	cmd := newFinderCommand(t, RealityFinderCommandRequest{Server: "203.0.113.10", Port: 443})
	body := `{
	  "network": {"asn": 197207, "org": "MCI\u0007", "country": "IR", "ip": "1.1.1.1"},
	  "warning": "<script>", "tested": 253, "port": 443, "filterTested": true,
	  "server": {"ip": "203.0.113.10", "asn": 24940, "holder": "HETZNER-AS", "prefix": "203.0.113.7/24"},
	  "targets": [
	    {"sni": "slow.example.com", "ip": "203.0.113.20", "score": 96, "ms": 400, "filtered": false},
	    {"sni": "Fast.Example.com", "ip": "203.0.113.21", "score": 96, "ms": 90, "filtered": false, "cdn": "evil"},
	    {"sni": "fast.example.com", "ip": "203.0.113.22", "score": 50, "ms": 10},
	    {"sni": "cut.example.com", "ip": "203.0.113.23", "score": 500, "ms": -4, "filtered": true},
	    {"sni": "untested.example.com", "ip": "203.0.113.24", "score": 40, "ms": 100},
	    {"sni": "not a name", "ip": "203.0.113.25", "score": 99},
	    {"sni": "v6.example.com", "ip": "2001:db8::1", "score": 99}
	  ]}`
	report, err := (&RealityFinderService{}).AddReport(cmd.Token, []byte(body), "5.120.1.2")
	if err != nil {
		t.Fatal(err)
	}
	if report.Network.Ip != "5.120.1.2" || report.Network.Org != "MCI" || report.Warning != "" || report.Server.Prefix != "203.0.113.0/24" {
		t.Errorf("report header = %+v / %+v / %q", report.Network, report.Server, report.Warning)
	}
	var got []string
	for _, tg := range report.Targets {
		filtered := "null"
		if tg.Filtered != nil {
			filtered = fmt.Sprint(*tg.Filtered)
		}
		got = append(got, fmt.Sprintf("%s %d %d %s %q", tg.Sni, tg.Score, tg.Ms, filtered, tg.Cdn))
	}
	want := []string{
		`cut.example.com 100 0 true ""`,
		`fast.example.com 96 90 false ""`,
		`slow.example.com 96 400 false ""`,
		`untested.example.com 40 100 null ""`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("targets:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRealityFinderKeepsTheLatestReports(t *testing.T) {
	resetRealityFinder(t, time.Unix(1_800_000_000, 0))
	cmd := newFinderCommand(t, RealityFinderCommandRequest{Server: "203.0.113.10", Port: 443})
	svc := &RealityFinderService{}
	for i := range realityFinderMaxReports + 3 {
		body, _ := json.Marshal(RealityFinderReport{Tested: i})
		if _, err := svc.AddReport(cmd.Token, body, "5.120.1.2"); err != nil {
			t.Fatal(err)
		}
	}
	reports, err := svc.Reports(cmd.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != realityFinderMaxReports || reports[0].Tested != 3 || reports[len(reports)-1].Tested != realityFinderMaxReports+2 {
		t.Errorf("kept %d reports, first tested=%d last tested=%d", len(reports), reports[0].Tested, reports[len(reports)-1].Tested)
	}
}

func TestRealityFinderCommandsAreCapped(t *testing.T) {
	clock := resetRealityFinder(t, time.Unix(1_800_000_000, 0))
	first := newFinderCommand(t, RealityFinderCommandRequest{Server: "203.0.113.10", Port: 443})
	for range realityFinderMaxCommands {
		*clock = clock.Add(time.Second)
		newFinderCommand(t, RealityFinderCommandRequest{Server: "203.0.113.10", Port: 443})
	}
	realityFinderJobsMu.Lock()
	n := len(realityFinderJobs)
	_, kept := realityFinderJobs[first.Token]
	realityFinderJobsMu.Unlock()
	if n != realityFinderMaxCommands || kept {
		t.Errorf("jobs = %d (oldest kept: %v), want %d with the oldest dropped", n, kept, realityFinderMaxCommands)
	}
}
