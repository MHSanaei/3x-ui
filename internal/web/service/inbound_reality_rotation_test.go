package service

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

const daySeconds = int64(86400)

func TestRealityRotationDue(t *testing.T) {
	now := int64(1_000_000_000)
	cases := []struct {
		name     string
		last     int64
		interval int
		want     bool
	}{
		{"disabled zero interval", now - 10*daySeconds, 0, false},
		{"disabled negative interval", now - 10*daySeconds, -3, false},
		{"not yet due", now - 1*daySeconds, 3, false},
		{"exactly due", now - 3*daySeconds, 3, true},
		{"overdue", now - 30*daySeconds, 3, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := realityRotationDue(c.last, c.interval, now); got != c.want {
				t.Fatalf("realityRotationDue(%d,%d,%d) = %v, want %v", c.last, c.interval, now, got, c.want)
			}
		})
	}
}

func TestFreshShortIds(t *testing.T) {
	ids := freshShortIds()
	if len(ids) != len(shortIdLengths) {
		t.Fatalf("freshShortIds len = %d, want %d", len(ids), len(shortIdLengths))
	}
	hexOnly := regexp.MustCompile(`^[0-9a-f]+$`)
	for i, id := range ids {
		if len(id) != shortIdLengths[i] {
			t.Fatalf("shortId %d len = %d, want %d", i, len(id), shortIdLengths[i])
		}
		if !hexOnly.MatchString(id) {
			t.Fatalf("shortId %q is not lowercase hex", id)
		}
	}
	// A second draw should differ (non-constant generator).
	if strings.Join(ids, ",") == strings.Join(freshShortIds(), ",") {
		t.Fatal("freshShortIds produced identical sets twice")
	}
}

func realityStream(t *testing.T, shortIdDays, publicKeyDays int, lastShortId, lastPubKey int64, shortIds []string) string {
	t.Helper()
	stream := map[string]any{
		"security": "reality",
		"realitySettings": map[string]any{
			"privateKey": "OLD_PRIV",
			"shortIds":   shortIds,
			"settings":   map[string]any{"publicKey": "OLD_PUB"},
			"rotation": map[string]any{
				"shortIdDays":           shortIdDays,
				"publicKeyDays":         publicKeyDays,
				"lastShortIdRotation":   lastShortId,
				"lastPublicKeyRotation": lastPubKey,
			},
		},
	}
	b, err := json.Marshal(stream)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func parseReality(t *testing.T, streamJSON string) map[string]any {
	t.Helper()
	var stream map[string]any
	if err := json.Unmarshal([]byte(streamJSON), &stream); err != nil {
		t.Fatal(err)
	}
	return stream["realitySettings"].(map[string]any)
}

func stubKeypair() (string, string, error) { return "NEW_PRIV", "NEW_PUB", nil }

func TestRotateRealityStreamSettings(t *testing.T) {
	now := int64(2_000_000_000)

	t.Run("shortId due replaces ids and restarts", func(t *testing.T) {
		in := realityStream(t, 3, 0, now-3*daySeconds, 0, []string{"aa", "bbbb"})
		out, changed, restart, err := RotateRealityStreamSettings(in, now, stubKeypair)
		if err != nil || !changed || !restart {
			t.Fatalf("changed=%v restart=%v err=%v", changed, restart, err)
		}
		reality := parseReality(t, out)
		ids := reality["shortIds"].([]any)
		if len(ids) != len(shortIdLengths) {
			t.Fatalf("shortIds len = %d, want %d", len(ids), len(shortIdLengths))
		}
		// A fresh full set replaced the 2-entry input: every id is hex of the
		// expected length. (Asserting a specific value like ids[0] != "aa" is
		// flaky — a random 2-char hex string is "aa" 1/256 of the time.)
		hexOnly := regexp.MustCompile(`^[0-9a-f]+$`)
		for i, raw := range ids {
			id := raw.(string)
			if len(id) != shortIdLengths[i] || !hexOnly.MatchString(id) {
				t.Fatalf("rotated shortId %d = %q, want %d hex chars", i, id, shortIdLengths[i])
			}
		}
		rot := reality["rotation"].(map[string]any)
		if int64(rot["lastShortIdRotation"].(float64)) != now {
			t.Fatal("lastShortIdRotation not advanced to now")
		}
	})

	t.Run("shortId not due leaves stream unchanged", func(t *testing.T) {
		in := realityStream(t, 3, 0, now-1*daySeconds, 0, []string{"aa", "bbbb"})
		out, changed, restart, err := RotateRealityStreamSettings(in, now, stubKeypair)
		if err != nil || changed || restart {
			t.Fatalf("expected no change: changed=%v restart=%v err=%v", changed, restart, err)
		}
		if out != in {
			t.Fatal("stream JSON mutated despite no rotation")
		}
	})

	t.Run("first sight anchors without rotating", func(t *testing.T) {
		in := realityStream(t, 3, 0, 0, 0, []string{"aa", "bbbb"})
		out, changed, restart, err := RotateRealityStreamSettings(in, now, stubKeypair)
		if err != nil || !changed || restart {
			t.Fatalf("expected anchor-only: changed=%v restart=%v err=%v", changed, restart, err)
		}
		reality := parseReality(t, out)
		if reality["shortIds"].([]any)[0].(string) != "aa" {
			t.Fatal("shortIds rotated on first sight; should only anchor")
		}
		rot := reality["rotation"].(map[string]any)
		if int64(rot["lastShortIdRotation"].(float64)) != now {
			t.Fatal("anchor not set to now")
		}
	})

	t.Run("publicKey due regenerates keypair", func(t *testing.T) {
		in := realityStream(t, 0, 7, 0, now-7*daySeconds, []string{"aa"})
		out, changed, restart, err := RotateRealityStreamSettings(in, now, stubKeypair)
		if err != nil || !changed || !restart {
			t.Fatalf("changed=%v restart=%v err=%v", changed, restart, err)
		}
		reality := parseReality(t, out)
		if reality["privateKey"].(string) != "NEW_PRIV" {
			t.Fatal("privateKey not rotated")
		}
		if reality["settings"].(map[string]any)["publicKey"].(string) != "NEW_PUB" {
			t.Fatal("publicKey not rotated")
		}
	})

	t.Run("disabled rotation untouched", func(t *testing.T) {
		in := realityStream(t, 0, 0, 0, 0, []string{"aa"})
		out, changed, restart, err := RotateRealityStreamSettings(in, now, stubKeypair)
		if err != nil || changed || restart || out != in {
			t.Fatalf("disabled rotation should be a no-op: changed=%v restart=%v err=%v", changed, restart, err)
		}
	})

	t.Run("non-reality stream ignored", func(t *testing.T) {
		in := `{"security":"tls","tlsSettings":{}}`
		out, changed, _, err := RotateRealityStreamSettings(in, now, stubKeypair)
		if err != nil || changed || out != in {
			t.Fatalf("tls stream should be ignored: changed=%v err=%v", changed, err)
		}
	})

	t.Run("empty stream ignored", func(t *testing.T) {
		out, changed, _, err := RotateRealityStreamSettings("", now, stubKeypair)
		if err != nil || changed || out != "" {
			t.Fatalf("empty stream should be ignored: changed=%v err=%v", changed, err)
		}
	})
}
