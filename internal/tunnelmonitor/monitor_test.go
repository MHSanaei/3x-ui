package tunnelmonitor

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestMonitorRestartsAfterFailureThreshold(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		URL:              "http://example.test",
		Interval:         time.Minute,
		Timeout:          time.Second,
		FailureThreshold: 2,
		Cooldown:         time.Minute,
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("tunnel down")
		}),
	}

	restarts := 0
	monitor := newWithClient(cfg, client, func(ctx context.Context) error {
		restarts++
		return nil
	})

	monitor.now = func() time.Time {
		return time.Unix(100, 0)
	}

	if recovered, _ := monitor.Step(context.Background()); recovered {
		t.Fatal("first failure must not trigger recovery")
	}

	if recovered, _ := monitor.Step(context.Background()); !recovered {
		t.Fatal("second consecutive failure should trigger recovery")
	}

	if restarts != 1 {
		t.Fatalf("expected 1 recovery, got %d", restarts)
	}
}

func TestMonitorRespectsRecoveryCooldown(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		URL:              "http://example.test",
		Interval:         time.Minute,
		Timeout:          time.Second,
		FailureThreshold: 1,
		Cooldown:         time.Minute,
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("tunnel down")
		}),
	}

	now := time.Unix(100, 0)
	restarts := 0

	monitor := newWithClient(cfg, client, func(ctx context.Context) error {
		restarts++
		return nil
	})

	monitor.now = func() time.Time {
		return now
	}

	recovered, _ := monitor.Step(context.Background())
	if !recovered {
		t.Fatal("first failure should trigger recovery when threshold is 1")
	}

	recovered, _ = monitor.Step(context.Background())
	if recovered {
		t.Fatal("cooldown should suppress immediate second recovery")
	}

	if restarts != 1 {
		t.Fatalf("expected 1 recovery during cooldown, got %d", restarts)
	}

	now = now.Add(time.Minute + time.Second)

	recovered, _ = monitor.Step(context.Background())
	if !recovered {
		t.Fatal("recovery should be allowed after cooldown")
	}

	if restarts != 2 {
		t.Fatalf("expected 2 recoveries after cooldown, got %d", restarts)
	}
}

func TestMonitorSuccessResetsFailures(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		URL:              "http://example.test",
		Interval:         time.Minute,
		Timeout:          time.Second,
		FailureThreshold: 2,
		Cooldown:         time.Minute,
	}

	fail := true
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if fail {
				return nil, errors.New("tunnel down")
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
			}, nil
		}),
	}

	restarts := 0
	monitor := newWithClient(cfg, client, func(ctx context.Context) error {
		restarts++
		return nil
	})

	_, _ = monitor.Step(context.Background())

	fail = false
	if recovered, err := monitor.Step(context.Background()); recovered || err != nil {
		t.Fatalf("successful probe should not recover or fail, recovered=%v err=%v", recovered, err)
	}

	fail = true
	if recovered, _ := monitor.Step(context.Background()); recovered {
		t.Fatal("failure after success should be counted as first failure again")
	}

	if restarts != 0 {
		t.Fatalf("expected no recovery, got %d", restarts)
	}
}

func TestConfigFromEnvParsesValues(t *testing.T) {
	t.Setenv("XUI_TUNNEL_HEALTH_MONITOR", "true")
	t.Setenv("XUI_TUNNEL_HEALTH_URL", "https://example.com/health")
	t.Setenv("XUI_TUNNEL_HEALTH_PROXY", "socks5://127.0.0.1:1080")
	t.Setenv("XUI_TUNNEL_HEALTH_INTERVAL", "15s")
	t.Setenv("XUI_TUNNEL_HEALTH_TIMEOUT", "3s")
	t.Setenv("XUI_TUNNEL_HEALTH_FAILURES", "4")
	t.Setenv("XUI_TUNNEL_HEALTH_COOLDOWN", "2m")

	cfg := ConfigFromEnv()

	if !cfg.Enabled {
		t.Fatal("expected monitor to be enabled")
	}

	if cfg.URL != "https://example.com/health" {
		t.Fatalf("unexpected URL: %s", cfg.URL)
	}

	if !strings.HasPrefix(cfg.ProxyURL, "socks5://") {
		t.Fatalf("unexpected proxy URL: %s", cfg.ProxyURL)
	}

	if cfg.Interval != 15*time.Second {
		t.Fatalf("unexpected interval: %s", cfg.Interval)
	}

	if cfg.Timeout != 3*time.Second {
		t.Fatalf("unexpected timeout: %s", cfg.Timeout)
	}

	if cfg.FailureThreshold != 4 {
		t.Fatalf("unexpected threshold: %d", cfg.FailureThreshold)
	}

	if cfg.Cooldown != 2*time.Minute {
		t.Fatalf("unexpected cooldown: %s", cfg.Cooldown)
	}
}
