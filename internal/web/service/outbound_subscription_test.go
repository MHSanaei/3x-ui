package service

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
)

func TestOutboundSubscriptionCreatePropagatesAllocationDatabaseFailures(t *testing.T) {
	setupSettingTestDB(t)
	db := database.GetDB()
	const callback = "test:fail_outbound_subscription_query"
	errInjected := errors.New("injected outbound subscription query failure")
	if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "outbound_subscriptions" {
			tx.AddError(errInjected)
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Query().Remove(callback); err != nil {
			t.Errorf("remove query callback: %v", err)
		}
	})

	for _, tc := range []struct {
		name      string
		tagPrefix string
		operation string
	}{
		{name: "default prefix query", tagPrefix: "", operation: "prefix allocation"},
		{name: "priority count query", tagPrefix: "custom-", operation: "priority allocation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created, err := (&OutboundSubscriptionService{}).Create("test", "https://1.1.1.1/sub", tc.tagPrefix, true, 600, false, false, false)
			if !errors.Is(err, errInjected) {
				t.Fatalf("Create error = %v, want injected %s query failure", err, tc.operation)
			}
			if created != nil {
				t.Fatalf("Create returned row %+v after %s query failure", created, tc.operation)
			}
		})
	}
}

func TestOutboundSubscriptionUpdatePropagatesPrefixQueryFailureWithoutMutation(t *testing.T) {
	setupSettingTestDB(t)
	db := database.GetDB()
	original := &model.OutboundSubscription{
		Remark: "before", Url: "https://1.1.1.1/original", TagPrefix: "custom-",
		Enabled: true, UpdateInterval: 600,
	}
	if err := db.Create(original).Error; err != nil {
		t.Fatalf("seed subscription: %v", err)
	}

	errInjected := errors.New("injected update prefix query failure")
	queryCount := 0
	const callback = "test:fail_update_prefix_query"
	if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "outbound_subscriptions" {
			return
		}
		queryCount++
		if queryCount == 2 {
			tx.AddError(errInjected)
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Query().Remove(callback); err != nil {
			t.Errorf("remove query callback: %v", err)
		}
	})

	err := (&OutboundSubscriptionService{}).Update(
		original.Id, "after", "https://1.1.1.1/changed", "", false, 1200, false, false, false,
	)
	if !errors.Is(err, errInjected) {
		t.Fatalf("Update error = %v, want injected prefix query failure", err)
	}
	if queryCount != 2 {
		t.Fatalf("outbound subscription queries = %d, want Get plus prefix allocation", queryCount)
	}

	var got model.OutboundSubscription
	if err := db.First(&got, original.Id).Error; err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	if got.Remark != original.Remark || got.Url != original.Url || got.TagPrefix != original.TagPrefix ||
		got.Enabled != original.Enabled || got.UpdateInterval != original.UpdateInterval {
		t.Fatalf("subscription changed after failed allocation: got %+v, want %+v", got, *original)
	}
}

func TestReadBoundedOutboundSubscriptionBody(t *testing.T) {
	t.Run("accepts body at the limit", func(t *testing.T) {
		want := bytes.Repeat([]byte("a"), int(maxOutboundSubscriptionBytes))
		got, err := readBoundedOutboundSubscriptionBody(bytes.NewReader(want))
		if err != nil {
			t.Fatalf("readBoundedOutboundSubscriptionBody: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("body mismatch: got %d bytes, want %d", len(got), len(want))
		}
	})

	t.Run("rejects body over the limit", func(t *testing.T) {
		body := bytes.Repeat([]byte("b"), int(maxOutboundSubscriptionBytes)+1)
		got, err := readBoundedOutboundSubscriptionBody(bytes.NewReader(body))
		if !errors.Is(err, errOutboundSubscriptionBodyTooLarge) {
			t.Fatalf("error = %v, want errOutboundSubscriptionBodyTooLarge", err)
		}
		if got != nil {
			t.Fatalf("oversized body returned %d bytes, want nil", len(got))
		}
	})
}

func TestDefaultPrefixNumber(t *testing.T) {
	mk := func(id int, prefix string) *model.OutboundSubscription {
		return &model.OutboundSubscription{Id: id, TagPrefix: prefix}
	}
	cases := []struct {
		name      string
		subs      []*model.OutboundSubscription
		excludeId int
		want      int
	}{
		{"no subscriptions starts at 1", nil, 0, 1},
		{"sequential prefixes give the next", []*model.OutboundSubscription{mk(1, "sub1-"), mk(2, "sub2-")}, 0, 3},
		{"reuses the lowest freed number", []*model.OutboundSubscription{mk(2, "sub2-")}, 0, 1},
		{"legacy blank prefix reserves its id", []*model.OutboundSubscription{mk(1, ""), mk(5, "sub3-")}, 0, 2},
		{"custom prefixes are ignored", []*model.OutboundSubscription{mk(1, "hk-"), mk(2, "jp-")}, 0, 1},
		{"excludes the edited subscription", []*model.OutboundSubscription{mk(5, "sub2-")}, 5, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := defaultPrefixNumber(c.subs, c.excludeId); got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestAssignStableTags(t *testing.T) {
	t.Run("reuses the tag mapped to a known identity", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "JP-Tokyo"}}
		prev := map[string]string{"id-abc": "sub1-keepme"}
		got := assignStableTags(parsed, []string{"id-abc"}, prev, nil, 1, "")
		if got[0] != "sub1-keepme" {
			t.Fatalf("got %q, want sub1-keepme", got[0])
		}
		if parsed[0]["tag"] != "sub1-keepme" {
			t.Fatalf("tag was not written back into the outbound: %v", parsed[0]["tag"])
		}
	})

	t.Run("falls back to the previous tag at the same position", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "JP-Tokyo"}}
		prev := map[string]string{"id-gone": "sub1-oldpos"}
		got := assignStableTags(parsed, []string{"id-new"}, prev, map[int]string{0: "sub1-oldpos"}, 1, "")
		if got[0] != "sub1-oldpos" {
			t.Fatalf("got %q, want sub1-oldpos", got[0])
		}
	})

	t.Run("does not let an inserted link steal a stable tag", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Poland"}, {"tag": "NewServer"}, {"tag": "Netherlands"}}
		prev := map[string]string{
			"id-poland":      "sub1-poland",
			"id-netherlands": "sub1-netherlands",
		}
		prevTagByIndex := map[int]string{0: "sub1-poland", 1: "sub1-netherlands"}

		got := assignStableTags(parsed, []string{"id-poland", "id-new", "id-netherlands"}, prev, prevTagByIndex, 1, "")
		want := []string{"sub1-poland", "sub1-newserver", "sub1-netherlands"}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("does not let a fresh tag steal a stable tag", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Netherlands"}, {"tag": "Renamed"}}
		prev := map[string]string{"id-netherlands": "sub1-netherlands"}

		got := assignStableTags(parsed, []string{"id-new", "id-netherlands"}, prev, nil, 1, "")
		want := []string{"sub1-netherlands-1", "sub1-netherlands"}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("skips reserved tags while adding a suffix", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Netherlands"}, {"tag": "First"}, {"tag": "Second"}}
		prev := map[string]string{
			"id-first":  "sub1-netherlands",
			"id-second": "sub1-netherlands-1",
		}

		got := assignStableTags(parsed, []string{"id-new", "id-first", "id-second"}, prev, nil, 1, "")
		want := []string{"sub1-netherlands-2", "sub1-netherlands", "sub1-netherlands-1"}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("allocates a fresh tag with the default sub<id>- prefix", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Tokyo"}}
		got := assignStableTags(parsed, []string{"id-x"}, nil, nil, 7, "")
		want := link.SuggestTag("sub7-", "Tokyo", 0)
		if got[0] != want {
			t.Fatalf("got %q, want %q", got[0], want)
		}
	})

	t.Run("uses a custom prefix for fresh tags", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Tokyo"}}
		got := assignStableTags(parsed, []string{"id-x"}, nil, nil, 1, "hk-")
		want := link.SuggestTag("hk-", "Tokyo", 0)
		if got[0] != want {
			t.Fatalf("got %q, want %q", got[0], want)
		}
	})

	t.Run("disambiguates colliding tags with a -N suffix", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Same"}, {"tag": "Same"}}
		got := assignStableTags(parsed, []string{"id1", "id2"}, nil, nil, 1, "p-")
		base := link.SuggestTag("p-", "Same", 0)
		if got[0] != base {
			t.Fatalf("got[0] = %q, want %q", got[0], base)
		}
		if got[1] != base+"-1" {
			t.Fatalf("got[1] = %q, want %q", got[1], base+"-1")
		}
	})
}

// TestOutboundsContainTag covers the guard that ensures the outbound under test
// is present in the HTTP-probe config. Subscription outbounds aren't part of the
// template outbounds the frontend sends as allOutbounds, so the probe must append
// the tested outbound when its tag is missing (otherwise burstObservatory has
// nothing to probe and every subscription test times out).
func TestOutboundsContainTag(t *testing.T) {
	template := []any{
		map[string]any{"tag": "direct", "protocol": "freedom"},
		map[string]any{"tag": "blocked", "protocol": "blackhole"},
	}
	if !outboundsContainTag(template, "direct") {
		t.Fatal("expected tag 'direct' to be found")
	}
	if outboundsContainTag(template, "sub1-tokyo") {
		t.Fatal("expected subscription tag to be absent from template outbounds")
	}
	if outboundsContainTag(nil, "anything") {
		t.Fatal("expected empty slice to contain no tags")
	}
	// Tolerates non-map / untagged entries without panicking.
	mixed := []any{"not-a-map", map[string]any{"protocol": "freedom"}}
	if outboundsContainTag(mixed, "direct") {
		t.Fatal("expected no match among untagged/non-map entries")
	}
}

// TestSanitizePublicHTTPURLRejectsPrivateAndBadSchemes covers the SSRF guard used
// when fetching subscription URLs. All rejected cases use literal IPs or bad
// schemes so the test never performs real DNS resolution.
func TestSanitizePublicHTTPURLRejectsPrivateAndBadSchemes(t *testing.T) {
	rejected := []string{
		"http://127.0.0.1/sub",                    // loopback
		"http://10.0.0.1/x",                       // private
		"http://192.168.1.1",                      // private
		"http://169.254.169.254/latest/meta-data", // link-local (cloud metadata)
		"http://[::1]:8080/sub",                   // IPv6 loopback
		"http://0.0.0.0",                          // unspecified
		"ftp://example.com/x",                     // unsupported scheme
		"file:///etc/passwd",                      // unsupported scheme
	}
	for _, raw := range rejected {
		if _, err := SanitizePublicHTTPURL(raw, false); err == nil {
			t.Errorf("expected %q to be rejected, got nil error", raw)
		}
	}

	t.Run("allows a public literal IP without DNS", func(t *testing.T) {
		got, err := SanitizePublicHTTPURL("http://8.8.8.8/sub", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "http://8.8.8.8/sub" {
			t.Fatalf("got %q, want http://8.8.8.8/sub", got)
		}
	})
}

// outboundsContainTag mirrors the small helper in the outbound subpackage so
// these subscription tests can assert on tag presence without importing it.
func outboundsContainTag(outbounds []any, tag string) bool {
	for _, ob := range outbounds {
		if m, ok := ob.(map[string]any); ok {
			if t, _ := m["tag"].(string); t == tag {
				return true
			}
		}
	}
	return false
}
