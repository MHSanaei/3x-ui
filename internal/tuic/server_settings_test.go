package tuic

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func TestNormalizeCongestionControl(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantValid bool
	}{
		{name: "default", want: "bbr", wantValid: true},
		{name: "bbr", input: "bbr", want: "bbr", wantValid: true},
		{name: "cubic", input: "cubic", want: "cubic", wantValid: true},
		{name: "new reno", input: "new_reno", want: "new_reno", wantValid: true},
		{name: "reno alias", input: "reno", want: "new_reno", wantValid: true},
		{name: "unknown falls back to reno", input: "quic", want: "new_reno"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := normalizeCongestionControl(test.input)
			if got != test.want || valid != test.wantValid {
				t.Fatalf("normalizeCongestionControl(%q) = (%q, %t), want (%q, %t)", test.input, got, valid, test.want, test.wantValid)
			}
		})
	}
}

func TestLogfUsesCommonLoggerAndHonorsThreshold(t *testing.T) {
	tests := []struct {
		level string
		want  []string
	}{
		{level: "debug", want: []string{"debug", "info", "warn", "error"}},
		{level: "info", want: []string{"info", "warn", "error"}},
		{level: "warn", want: []string{"warn", "error"}},
		{level: "error", want: []string{"error"}},
	}

	for _, test := range tests {
		t.Run(test.level, func(t *testing.T) {
			marker := fmt.Sprintf("tuic-log-%s-%d", test.level, time.Now().UnixNano())
			server := &Server{id: 99001}
			server.updateRuntimeSettings("log-test", "bbr", test.level)
			for _, level := range []struct {
				name  string
				value uint32
			}{{"debug", tuicLogDebug}, {"info", tuicLogInfo}, {"warn", tuicLogWarn}, {"error", tuicLogError}} {
				server.logf(level.value, "%s-%s", marker, level.name)
			}

			logs := strings.Join(logger.GetLogs(10000, "DEBUG"), "\n")
			for _, name := range []string{"debug", "info", "warn", "error"} {
				want := slices.Contains(test.want, name)
				got := strings.Contains(logs, marker+"-"+name)
				if got != want {
					t.Errorf("log level %s present = %t, want %t", name, got, want)
				}
			}
			if !strings.Contains(logs, "inbound 99001 (log-test)") {
				t.Fatal("TUIC event was not written through the shared 3x-ui logger")
			}
		})
	}
}

func TestLogLevelsAreIsolatedPerInboundAndUpdateLive(t *testing.T) {
	marker := fmt.Sprintf("tuic-log-isolation-%d", time.Now().UnixNano())
	debugInbound := &Server{id: 99011}
	errorInbound := &Server{id: 99012}
	debugInbound.updateRuntimeSettings("debug-inbound", "bbr", "debug")
	errorInbound.updateRuntimeSettings("error-inbound", "bbr", "error")
	debugInbound.logf(tuicLogInfo, "%s-debug", marker)
	errorInbound.logf(tuicLogInfo, "%s-hidden", marker)
	debugInbound.UpdateRuntimeSettings("debug-inbound", "bbr", "warn")
	debugInbound.logf(tuicLogInfo, "%s-hidden-after-update", marker)
	debugInbound.logf(tuicLogWarn, "%s-warn-after-update", marker)

	logs := strings.Join(logger.GetLogs(10000, "DEBUG"), "\n")
	if !strings.Contains(logs, marker+"-debug") || !strings.Contains(logs, marker+"-warn-after-update") {
		t.Fatal("expected permitted events from debug inbound")
	}
	if strings.Contains(logs, marker+"-hidden") || strings.Contains(logs, marker+"-hidden-after-update") {
		t.Fatal("a TUIC inbound emitted an event below its own log threshold")
	}
}

func TestEnsureStartupFailureHonorsInboundLogThreshold(t *testing.T) {
	tests := []struct {
		level string
		want  bool
	}{
		{level: "error", want: false},
		{level: "warn", want: true},
	}
	for _, test := range tests {
		t.Run(test.level, func(t *testing.T) {
			marker := fmt.Sprintf("tuic-start-failure-%s-%d", test.level, time.Now().UnixNano())
			manager := &Manager{servers: map[int]*managed{}, lastStartErr: map[int]string{}, pendingTraffic: map[string]ClientTrafficDelta{}}
			err := manager.Ensure(Instance{
				Id:                99031,
				Tag:               marker,
				Listen:            "127.0.0.1",
				Port:              0,
				LogLevel:          test.level,
				CongestionControl: "bbr",
				Clients:           []TuicClientSettings{{UUID: "a0000000-0000-0000-0000-000000000031", Password: "p", Email: "startup@x"}},
			})
			if err == nil {
				t.Fatal("Ensure unexpectedly started without a certificate")
			}

			logs := strings.Join(logger.GetLogs(10000, "DEBUG"), "\n")
			got := strings.Contains(logs, marker+"): failed to start server")
			if got != test.want {
				t.Fatalf("startup warning logged = %t, want %t", got, test.want)
			}
		})
	}
}
