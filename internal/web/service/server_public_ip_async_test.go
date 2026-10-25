package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

// A box with no IPv6 route spends 3s per lookup service, and a status sample
// that waits for that is a panel reporting nothing for the first ~15s.
func TestStatusSampleDoesNotWaitOnPublicIPLookup(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte("203.0.113.7"))
	}))
	t.Cleanup(srv.Close)

	savedV4, savedV6 := publicIPv4Services, publicIPv6Services
	publicIPv4Services = []string{srv.URL}
	publicIPv6Services = []string{srv.URL}
	t.Cleanup(func() { publicIPv4Services, publicIPv6Services = savedV4, savedV6 })

	svc := &ServerService{}
	status := svc.CurrentStatus()
	if status == nil {
		t.Fatal("CurrentStatus returned nil while the IP lookup was in flight")
	}
	if status.PublicIP.IPv4 != "" {
		t.Fatalf("the sample waited for the lookup: PublicIP.IPv4 = %q, want it still unresolved", status.PublicIP.IPv4)
	}

	close(release)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if ipv4, _ := svc.publicIPs(); ipv4 == "203.0.113.7" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the background lookup never cached the public IP")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// publicIPTestTransport keeps clock-driven tests entirely off the network.
type publicIPTestTransport func(*http.Request) (*http.Response, error)

func (f publicIPTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stubPublicIPServices(t *testing.T, respond publicIPTestTransport) {
	t.Helper()
	oldTransport, old4, old6 := http.DefaultTransport, publicIPv4Services, publicIPv6Services
	http.DefaultTransport = respond
	publicIPv4Services = []string{"https://ipv4.example.test/primary", "https://ipv4.example.test/fallback"}
	publicIPv6Services = []string{"https://ipv6.example.test/primary", "https://ipv6.example.test/fallback"}
	t.Cleanup(func() { http.DefaultTransport, publicIPv4Services, publicIPv6Services = oldTransport, old4, old6 })
}

func publicIPResponse(status int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
}

func assertCachedPublicIPs(t *testing.T, svc *ServerService, want4, want6 string) {
	t.Helper()
	got4, got6 := svc.publicIPs()
	if got4 != want4 || got6 != want6 {
		t.Fatalf("cached IPs=(%q,%q), want (%q,%q)", got4, got6, want4, want6)
	}
}

func TestPublicIPRefreshesAfterExpiryWithoutRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		value4, value6 := "192.0.2.10", "2001:db8::10"
		stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if r.URL.Hostname() == "ipv4.example.test" {
				return publicIPResponse(200, value4)
			}
			return publicIPResponse(200, value6)
		})
		svc := &ServerService{}
		svc.resolvePublicIPs()
		assertCachedPublicIPs(t, svc, value4, value6)
		value4, value6 = "192.0.2.20", "2001:db8::20"
		time.Sleep(5*time.Minute - time.Nanosecond)
		svc.resolvePublicIPsInBackground()
		synctest.Wait()
		if got := calls.Load(); got != 2 {
			t.Fatalf("lookup before expiry: %d requests", got)
		}
		time.Sleep(time.Nanosecond)
		svc.resolvePublicIPsInBackground()
		synctest.Wait()
		assertCachedPublicIPs(t, svc, value4, value6)
		if got := calls.Load(); got != 4 {
			t.Fatalf("refresh requests=%d, want 4", got)
		}
	})
}

func TestPublicIPFailedRefreshRetainsLastKnownAndRetries(t *testing.T) {
	for _, failure := range []string{"http-error", "empty-body", "transport-error", "timeout"} {
		t.Run(failure, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls atomic.Int32
				unavailable := false
				stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
					calls.Add(1)
					if unavailable {
						switch failure {
						case "empty-body":
							return publicIPResponse(200, "")
						case "transport-error":
							return nil, io.ErrUnexpectedEOF
						case "timeout":
							<-r.Context().Done()
							return nil, r.Context().Err()
						default:
							return publicIPResponse(503, "")
						}
					}
					if r.URL.Hostname() == "ipv4.example.test" {
						return publicIPResponse(200, "192.0.2.10")
					}
					return publicIPResponse(200, "2001:db8::10")
				})
				svc := &ServerService{}
				svc.resolvePublicIPs()
				unavailable = true
				time.Sleep(5 * time.Minute)
				// Await the refresh, including HTTP timeouts, before checking its result.
				done := svc.resolvePublicIPsInBackground()
				if done == nil {
					t.Fatal("expired cache did not start a refresh")
				}
				<-done
				assertCachedPublicIPs(t, svc, "192.0.2.10", "2001:db8::10")
				if got := calls.Load(); got != 6 {
					t.Fatalf("failed refresh made %d requests, want 6 including fallbacks", got)
				}
				time.Sleep(time.Minute - time.Nanosecond)
				svc.resolvePublicIPsInBackground()
				synctest.Wait()
				if got := calls.Load(); got != 6 {
					t.Fatalf("retried too early: %d requests", got)
				}
				unavailable = false
				time.Sleep(time.Nanosecond)
				svc.resolvePublicIPsInBackground()
				synctest.Wait()
				if got := calls.Load(); got != 8 {
					t.Fatalf("retry requests=%d, want 8", got)
				}
			})
		})
	}
}

func TestPublicIPInitialFailureRecoversBothFamilies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		available := false
		var calls atomic.Int32
		stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if !available {
				return publicIPResponse(503, "")
			}
			if r.URL.Hostname() == "ipv4.example.test" {
				return publicIPResponse(200, "192.0.2.20")
			}
			return publicIPResponse(200, "2001:db8::20")
		})
		svc := &ServerService{}
		svc.resolvePublicIPs()
		assertCachedPublicIPs(t, svc, "N/A", "N/A")
		available = true
		time.Sleep(time.Minute)
		svc.resolvePublicIPsInBackground()
		synctest.Wait()
		assertCachedPublicIPs(t, svc, "192.0.2.20", "2001:db8::20")
		if got := calls.Load(); got != 6 {
			t.Fatalf("initial failure/recovery requests=%d, want 6", got)
		}
	})
}

func TestPublicIPFamiliesRefreshIndependently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls4, calls6 atomic.Int32
		ipv6Available := false
		stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
			if r.URL.Hostname() == "ipv4.example.test" {
				calls4.Add(1)
				return publicIPResponse(200, "192.0.2.10")
			}
			calls6.Add(1)
			if !ipv6Available {
				return publicIPResponse(503, "")
			}
			return publicIPResponse(200, "2001:db8::20")
		})
		svc := &ServerService{}
		svc.resolvePublicIPs()
		assertCachedPublicIPs(t, svc, "192.0.2.10", "N/A")
		ipv6Available = true
		time.Sleep(time.Minute)
		svc.resolvePublicIPsInBackground()
		synctest.Wait()
		assertCachedPublicIPs(t, svc, "192.0.2.10", "2001:db8::20")
		if calls4.Load() != 1 || calls6.Load() != 3 {
			t.Fatalf("family requests IPv4=%d IPv6=%d", calls4.Load(), calls6.Load())
		}
	})
}

func TestPublicIPRefreshIsSingleFlightAndKeepsCachedReadsAvailable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		slow := false
		release := make(chan struct{})
		stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if slow {
				select {
				case <-release:
				case <-r.Context().Done():
					return nil, r.Context().Err()
				}
			}
			if r.URL.Hostname() == "ipv4.example.test" {
				return publicIPResponse(200, "192.0.2.10")
			}
			return publicIPResponse(200, "2001:db8::10")
		})
		svc := &ServerService{}
		svc.resolvePublicIPs()
		slow = true
		time.Sleep(5 * time.Minute)
		svc.resolvePublicIPsInBackground()
		synctest.Wait()
		var wg sync.WaitGroup
		var completed atomic.Int32
		for range 20 {
			wg.Go(func() { svc.resolvePublicIPsInBackground(); svc.resolvePublicIPs(); completed.Add(1) })
		}
		synctest.Wait()
		// Cached reads must remain available while the HTTP request is blocked.
		assertCachedPublicIPs(t, svc, "192.0.2.10", "2001:db8::10")
		count := calls.Load()
		early := completed.Load()
		close(release)
		wg.Wait()
		synctest.Wait()
		if early != 20 {
			t.Fatalf("only %d warm callers returned while the lookup was blocked", early)
		}
		if completed.Load() != 20 {
			t.Fatalf("only %d synchronous callers completed", completed.Load())
		}
		if count != 3 {
			t.Fatalf("concurrent refresh started %d requests, want 3 including initial pair", count)
		}
	})
}

func TestPublicIPStatusUsesRefreshedAddressWithoutBlocking(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
	savedHistory := systemMetrics
	systemMetrics = newMetricHistory()
	t.Cleanup(func() { systemMetrics = savedHistory })
	synctest.Test(t, func(t *testing.T) {
		value4, value6 := "192.0.2.10", "2001:db8::10"
		slow := false
		release := make(chan struct{})
		stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
			if slow {
				select {
				case <-release:
				case <-r.Context().Done():
					return nil, r.Context().Err()
				}
			}
			if r.URL.Hostname() == "ipv4.example.test" {
				return publicIPResponse(200, value4)
			}
			return publicIPResponse(200, value6)
		})
		svc := &ServerService{}
		svc.resolvePublicIPs()
		svc.RefreshStatus()
		assertStatus := func(want string) {
			t.Helper()
			data, err := json.Marshal(svc.CurrentStatus())
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				PublicIP struct {
					IPv4 string `json:"ipv4"`
				} `json:"publicIP"`
			}
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.PublicIP.IPv4 != want {
				t.Fatalf("status JSON IPv4=%q, want %q", wire.PublicIP.IPv4, want)
			}
		}
		assertStatus(value4)
		value4, value6, slow = "192.0.2.20", "2001:db8::20", true
		time.Sleep(5 * time.Minute)
		start := time.Now()
		svc.RefreshStatus()
		if time.Since(start) != 0 {
			t.Fatal("status sample waited for a public-IP request")
		}
		synctest.Wait()
		assertStatus("192.0.2.10")
		close(release)
		synctest.Wait()
		svc.RefreshStatus()
		assertStatus("192.0.2.20")
	})
}

func TestPublicIPColdSynchronousCallersShareInitialLookup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls, completed atomic.Int32
		release := make(chan struct{})
		stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
			if r.URL.Hostname() == "ipv4.example.test" {
				return publicIPResponse(200, "192.0.2.10")
			}
			return publicIPResponse(200, "2001:db8::10")
		})
		svc := &ServerService{}
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() { svc.resolvePublicIPs(); completed.Add(1) })
		}
		synctest.Wait()
		early, started := completed.Load(), calls.Load()
		close(release)
		wg.Wait()
		synctest.Wait()
		if early != 0 || started != 1 {
			t.Fatalf("cold callers completed=%d started requests=%d, want 0 and 1", early, started)
		}
		assertCachedPublicIPs(t, svc, "192.0.2.10", "2001:db8::10")
		if calls.Load() != 2 || completed.Load() != 20 {
			t.Fatalf("after initial lookup requests=%d completed=%d", calls.Load(), completed.Load())
		}
	})
}

func waitForPublicIPLookup(t *testing.T, svc *ServerService) {
	t.Helper()
	if done := svc.resolvePublicIPsInBackground(); done != nil {
		<-done
	}
}

func TestPublicIPv6FailuresBackOffAndCapRetries(t *testing.T) {
	for _, failure := range []string{"http-error", "empty-body", "transport-error", "timeout"} {
		t.Run(failure, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls6 atomic.Int32
				stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
					if r.URL.Hostname() == "ipv4.example.test" {
						return publicIPResponse(http.StatusOK, "192.0.2.10")
					}
					calls6.Add(1)
					switch failure {
					case "empty-body":
						return publicIPResponse(http.StatusOK, "")
					case "transport-error":
						return nil, io.ErrUnexpectedEOF
					case "timeout":
						<-r.Context().Done()
						return nil, r.Context().Err()
					default:
						return publicIPResponse(http.StatusServiceUnavailable, "")
					}
				})
				svc := &ServerService{}
				svc.resolvePublicIPs()
				assertCachedPublicIPs(t, svc, "192.0.2.10", "N/A")
				if got := calls6.Load(); got != 2 {
					t.Fatalf("initial IPv6 requests=%d, want 2", got)
				}
				for _, delay := range []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 15 * time.Minute, 15 * time.Minute, 15 * time.Minute} {
					before := calls6.Load()
					time.Sleep(delay - time.Nanosecond)
					waitForPublicIPLookup(t, svc)
					if got := calls6.Load(); got != before {
						t.Fatalf("IPv6 retried before %v: requests=%d, want %d", delay, got, before)
					}
					time.Sleep(time.Nanosecond)
					waitForPublicIPLookup(t, svc)
					if got := calls6.Load(); got != before+2 {
						t.Fatalf("IPv6 requests at %v=%d, want %d", delay, got, before+2)
					}
					assertCachedPublicIPs(t, svc, "192.0.2.10", "N/A")
				}
			})
		})
	}
}

func TestPublicIPv6RecoveryResetsBackoffAndRefreshesNormally(t *testing.T) {
	for _, recovered := range []string{"2001:db8::10", "2001:db8::20"} {
		t.Run(recovered, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls6 atomic.Int32
				available := true
				value6 := "2001:db8::10"
				stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
					if r.URL.Hostname() == "ipv4.example.test" {
						return publicIPResponse(http.StatusOK, "192.0.2.10")
					}
					calls6.Add(1)
					if !available {
						return publicIPResponse(http.StatusServiceUnavailable, "")
					}
					return publicIPResponse(http.StatusOK, value6)
				})
				svc := &ServerService{}
				svc.resolvePublicIPs()
				available = false
				for _, delay := range []time.Duration{5 * time.Minute, time.Minute, 2 * time.Minute, 5 * time.Minute} {
					time.Sleep(delay)
					waitForPublicIPLookup(t, svc)
					assertCachedPublicIPs(t, svc, "192.0.2.10", "2001:db8::10")
				}
				available, value6 = true, recovered
				before := calls6.Load()
				time.Sleep(15*time.Minute - time.Nanosecond)
				waitForPublicIPLookup(t, svc)
				if got := calls6.Load(); got != before {
					t.Fatalf("recovery attempted before capped deadline: requests=%d, want %d", got, before)
				}
				time.Sleep(time.Nanosecond)
				waitForPublicIPLookup(t, svc)
				assertCachedPublicIPs(t, svc, "192.0.2.10", recovered)
				if got := calls6.Load(); got != before+1 {
					t.Fatalf("recovery requests=%d, want %d", got, before+1)
				}

				before = calls6.Load()
				time.Sleep(5*time.Minute - time.Nanosecond)
				waitForPublicIPLookup(t, svc)
				if got := calls6.Load(); got != before {
					t.Fatalf("successful IPv6 refreshed before five minutes: requests=%d, want %d", got, before)
				}
				available = false
				time.Sleep(time.Nanosecond)
				waitForPublicIPLookup(t, svc)
				if got := calls6.Load(); got != before+2 {
					t.Fatalf("refresh requests=%d, want %d", got, before+2)
				}
				assertCachedPublicIPs(t, svc, "192.0.2.10", recovered)
				before = calls6.Load()
				time.Sleep(time.Minute - time.Nanosecond)
				waitForPublicIPLookup(t, svc)
				if got := calls6.Load(); got != before {
					t.Fatalf("IPv6 retried before reset one-minute deadline: requests=%d, want %d", got, before)
				}
				available, value6 = true, "2001:db8::30"
				time.Sleep(time.Nanosecond)
				waitForPublicIPLookup(t, svc)
				assertCachedPublicIPs(t, svc, "192.0.2.10", value6)
				if got := calls6.Load(); got != before+1 {
					t.Fatalf("one-minute recovery requests=%d, want %d", got, before+1)
				}
			})
		})
	}
}

func TestPublicIPv6BackoffPreservesIPv4RefreshAndRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls4, calls6 atomic.Int32
		available4, value4 := true, "192.0.2.10"
		stubPublicIPServices(t, func(r *http.Request) (*http.Response, error) {
			if r.URL.Hostname() == "ipv4.example.test" {
				calls4.Add(1)
				if available4 {
					return publicIPResponse(http.StatusOK, value4)
				}
			} else {
				calls6.Add(1)
			}
			return publicIPResponse(http.StatusServiceUnavailable, "")
		})
		svc := &ServerService{}
		svc.resolvePublicIPs()
		for _, delay := range []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute} {
			time.Sleep(delay)
			waitForPublicIPLookup(t, svc)
		}
		if got := calls6.Load(); got != 8 {
			t.Fatalf("IPv6 requests before capped backoff=%d, want 8", got)
		}
		value4 = "192.0.2.20"
		before := calls4.Load()
		time.Sleep(5*time.Minute - time.Nanosecond)
		waitForPublicIPLookup(t, svc)
		if got := calls4.Load(); got != before {
			t.Fatalf("IPv4 refreshed before five minutes: requests=%d, want %d", got, before)
		}
		time.Sleep(time.Nanosecond)
		waitForPublicIPLookup(t, svc)
		assertCachedPublicIPs(t, svc, value4, "N/A")
		if got := calls4.Load(); got != before+1 {
			t.Fatalf("IPv4 refresh requests=%d, want %d", got, before+1)
		}
		available4 = false
		time.Sleep(5 * time.Minute)
		waitForPublicIPLookup(t, svc)
		assertCachedPublicIPs(t, svc, value4, "N/A")
		before = calls4.Load()
		time.Sleep(time.Minute - time.Nanosecond)
		waitForPublicIPLookup(t, svc)
		if got := calls4.Load(); got != before {
			t.Fatalf("IPv4 retried before one minute: requests=%d, want %d", got, before)
		}
		available4, value4 = true, "192.0.2.30"
		time.Sleep(time.Nanosecond)
		waitForPublicIPLookup(t, svc)
		assertCachedPublicIPs(t, svc, value4, "N/A")
		if got := calls4.Load(); got != before+1 {
			t.Fatalf("IPv4 retry requests=%d, want %d", got, before+1)
		}
		if got := calls6.Load(); got != 8 {
			t.Fatalf("IPv6 retried during capped backoff: requests=%d, want 8", got)
		}
	})
}
