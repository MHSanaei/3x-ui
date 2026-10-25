package service

import (
	"context"
	"errors"
	"reflect"
	"regexp"
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
	script, err := s.Script(cmd.Token, "https://panel.example.com/secret/cleanip/run/"+cmd.Token+"/report")
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
	if _, err := s.Script(cmd.Token, "https://p/r"); !errors.Is(err, errCleanIpExpired) {
		t.Errorf("Script err = %v, want errCleanIpExpired", err)
	}
}
