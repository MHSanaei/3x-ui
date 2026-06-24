package tunnelmonitor

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/op/go-logging"
)

func TestMain(m *testing.M) {
	logger.InitLogger(logging.ERROR)
	m.Run()
}

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

func failingClient() *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("tunnel down")
		}),
	}
}

func statusClient(code int) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: code, Body: http.NoBody}, nil
		}),
	}
}

func TestProbeStatusCodeClassification(t *testing.T) {
	cases := []struct {
		status  int
		healthy bool
	}{
		{199, false},
		{200, true},
		{204, true},
		{301, true},
		{399, true},
		{400, false},
		{404, false},
		{500, false},
	}

	for _, tc := range cases {
		cfg := Config{
			Enabled:          true,
			URL:              "http://example.test",
			Interval:         time.Minute,
			Timeout:          time.Second,
			FailureThreshold: 100,
			Cooldown:         time.Minute,
		}

		monitor := newWithClient(cfg, statusClient(tc.status), func(ctx context.Context) error {
			return nil
		})

		recovered, err := monitor.Step(context.Background())
		if recovered {
			t.Fatalf("status %d: unexpected recovery", tc.status)
		}
		if tc.healthy && err != nil {
			t.Fatalf("status %d: expected healthy probe, got error %v", tc.status, err)
		}
		if !tc.healthy && err == nil {
			t.Fatalf("status %d: expected failure, got nil error", tc.status)
		}
	}
}

func TestNormalizeClampsBounds(t *testing.T) {
	got := Config{
		URL:              "   ",
		Interval:         0,
		Timeout:          500 * time.Millisecond,
		FailureThreshold: 0,
		Cooldown:         0,
	}.Normalize()

	if got.URL != defaultHealthURL {
		t.Fatalf("URL not defaulted: %q", got.URL)
	}
	if got.Interval != defaultInterval {
		t.Fatalf("Interval not clamped: %s", got.Interval)
	}
	if got.Timeout != defaultTimeout {
		t.Fatalf("Timeout not clamped: %s", got.Timeout)
	}
	if got.FailureThreshold != defaultFailureThreshold {
		t.Fatalf("FailureThreshold not clamped: %d", got.FailureThreshold)
	}
	if got.Cooldown != defaultCooldown {
		t.Fatalf("Cooldown not clamped: %s", got.Cooldown)
	}

	valid := Config{
		URL:              "https://example.com/health",
		Interval:         15 * time.Second,
		Timeout:          3 * time.Second,
		FailureThreshold: 5,
		Cooldown:         2 * time.Minute,
	}.Normalize()

	if valid.URL != "https://example.com/health" ||
		valid.Interval != 15*time.Second ||
		valid.Timeout != 3*time.Second ||
		valid.FailureThreshold != 5 ||
		valid.Cooldown != 2*time.Minute {
		t.Fatalf("valid config was mutated: %+v", valid)
	}
}

func TestNewRejectsUnsupportedProxyScheme(t *testing.T) {
	m, err := New(Config{ProxyURL: "ftp://127.0.0.1:21"}, func(ctx context.Context) error {
		return nil
	})
	if err == nil || m != nil {
		t.Fatalf("expected error and nil monitor for bad scheme, got m=%v err=%v", m, err)
	}

	m, err = New(Config{}, func(ctx context.Context) error {
		return nil
	})
	if err != nil || m == nil {
		t.Fatalf("expected a valid monitor for empty proxy, got m=%v err=%v", m, err)
	}
}

func TestMonitorRecoveryErrorDoesNotArmCooldown(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		URL:              "http://example.test",
		Interval:         time.Minute,
		Timeout:          time.Second,
		FailureThreshold: 1,
		Cooldown:         time.Minute,
	}

	attempts := 0
	monitor := newWithClient(cfg, failingClient(), func(ctx context.Context) error {
		attempts++
		return errors.New("restart failed")
	})
	monitor.now = func() time.Time {
		return time.Unix(100, 0)
	}

	recovered, err := monitor.Step(context.Background())
	if recovered || err == nil {
		t.Fatalf("failed recovery must report recovered=false with an error, got recovered=%v err=%v", recovered, err)
	}
	if !monitor.lastRecovery.IsZero() {
		t.Fatal("a failed recovery must not arm the cooldown")
	}

	if _, err := monitor.Step(context.Background()); err == nil {
		t.Fatal("expected error on the second failing step")
	}
	if attempts != 2 {
		t.Fatalf("recovery should be retried (no cooldown) after a failure, attempts=%d", attempts)
	}
}

func TestMonitorNilRecoverStaysBounded(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		URL:              "http://example.test",
		Interval:         time.Minute,
		Timeout:          time.Second,
		FailureThreshold: 2,
		Cooldown:         time.Minute,
	}

	monitor := newWithClient(cfg, failingClient(), nil)

	for i := 0; i < 5; i++ {
		recovered, _ := monitor.Step(context.Background())
		if recovered {
			t.Fatal("a nil recovery func must never report recovery")
		}
		if monitor.failures > cfg.FailureThreshold {
			t.Fatalf("failures must stay capped at threshold %d, got %d", cfg.FailureThreshold, monitor.failures)
		}
	}
}

func TestMonitorFailuresCappedDuringCooldown(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		URL:              "http://example.test",
		Interval:         time.Minute,
		Timeout:          time.Second,
		FailureThreshold: 2,
		Cooldown:         time.Minute,
	}

	restarts := 0
	monitor := newWithClient(cfg, failingClient(), func(ctx context.Context) error {
		restarts++
		return nil
	})
	monitor.now = func() time.Time {
		return time.Unix(100, 0)
	}

	monitor.Step(context.Background())
	if recovered, _ := monitor.Step(context.Background()); !recovered {
		t.Fatal("expected recovery once the threshold is reached")
	}

	for i := 0; i < 6; i++ {
		monitor.Step(context.Background())
		if monitor.failures > cfg.FailureThreshold {
			t.Fatalf("failures must never exceed threshold %d during cooldown, got %d", cfg.FailureThreshold, monitor.failures)
		}
	}

	if restarts != 1 {
		t.Fatalf("cooldown should suppress further recoveries, restarts=%d", restarts)
	}
}

func TestMonitorRunStopsOnContextCancel(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		URL:              "http://example.test",
		Timeout:          time.Second,
		FailureThreshold: 1,
		Cooldown:         time.Hour,
	}

	recovered := make(chan struct{})
	var once sync.Once
	monitor := newWithClient(cfg, failingClient(), func(ctx context.Context) error {
		once.Do(func() { close(recovered) })
		return nil
	})
	monitor.cfg.Interval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		monitor.Run(ctx)
		close(done)
	}()

	select {
	case <-recovered:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("Run did not trigger recovery within the deadline")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
