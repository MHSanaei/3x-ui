package service

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"testing"
)

const realityRotationStream = `{
  "network": "tcp",
  "security": "reality",
  "realitySettings": {
    "target": "example.com:443",
    "shortIds": ["aa", "bbbb", "cccccc", "dddddddd"]
  }
}`

func sequenceShortIDGenerator(ids ...string) realityShortIDGenerator {
	index := 0
	return func() (string, error) {
		if index >= len(ids) {
			return "", errors.New("test generator exhausted")
		}
		id := ids[index]
		index++
		return id, nil
	}
}

func shortIDsFromStream(t *testing.T, streamSettings string) []string {
	t.Helper()
	_, _, ids, err := parseRealityShortIDs(streamSettings)
	if err != nil {
		t.Fatalf("parse rotated stream: %v", err)
	}
	return ids
}

func TestNewRealityShortID(t *testing.T) {
	first, err := newRealityShortID()
	if err != nil {
		t.Fatalf("newRealityShortID: %v", err)
	}
	second, err := newRealityShortID()
	if err != nil {
		t.Fatalf("newRealityShortID second call: %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(first) {
		t.Fatalf("generated short ID %q is not 16 lowercase hex digits", first)
	}
	if first == second {
		t.Fatalf("two generated short IDs are equal: %q", first)
	}
}

func TestRotateRealityShortIDs_All(t *testing.T) {
	rotation, err := rotateRealityShortIDs(
		realityRotationStream,
		0,
		0,
		0,
		sequenceShortIDGenerator("1111111111111111", "2222222222222222", "3333333333333333", "4444444444444444"),
	)
	if err != nil {
		t.Fatalf("rotateRealityShortIDs: %v", err)
	}

	wantActive := []string{"1111111111111111", "2222222222222222", "3333333333333333", "4444444444444444"}
	wantRetiring := []string{"aa", "bbbb", "cccccc", "dddddddd"}
	if !slices.Equal(rotation.ActiveIDs, wantActive) {
		t.Fatalf("active IDs = %v, want %v", rotation.ActiveIDs, wantActive)
	}
	if !slices.Equal(rotation.RetiringIDs, wantRetiring) {
		t.Fatalf("retiring IDs = %v, want %v", rotation.RetiringIDs, wantRetiring)
	}
	if got := shortIDsFromStream(t, rotation.StreamSettings); !slices.Equal(got, append(wantActive, wantRetiring...)) {
		t.Fatalf("accepted IDs = %v, want active + retiring", got)
	}
	if rotation.ActiveCount != 4 || rotation.NextCursor != 0 {
		t.Fatalf("state = activeCount %d, cursor %d; want 4, 0", rotation.ActiveCount, rotation.NextCursor)
	}
}

func TestRotateRealityShortIDs_SubsetUsesCursor(t *testing.T) {
	rotation, err := rotateRealityShortIDs(
		realityRotationStream,
		4,
		2,
		2,
		sequenceShortIDGenerator("1111111111111111", "2222222222222222"),
	)
	if err != nil {
		t.Fatalf("rotateRealityShortIDs: %v", err)
	}

	wantActive := []string{"aa", "bbbb", "1111111111111111", "2222222222222222"}
	wantRetiring := []string{"cccccc", "dddddddd"}
	if !slices.Equal(rotation.ActiveIDs, wantActive) {
		t.Fatalf("active IDs = %v, want %v", rotation.ActiveIDs, wantActive)
	}
	if !slices.Equal(rotation.RetiringIDs, wantRetiring) {
		t.Fatalf("retiring IDs = %v, want %v", rotation.RetiringIDs, wantRetiring)
	}
	if rotation.NextCursor != 0 {
		t.Fatalf("next cursor = %d, want 0", rotation.NextCursor)
	}
}

func TestRotateRealityShortIDs_SubsetWrapsCursor(t *testing.T) {
	rotation, err := rotateRealityShortIDs(
		realityRotationStream,
		4,
		3,
		2,
		sequenceShortIDGenerator("1111111111111111", "2222222222222222"),
	)
	if err != nil {
		t.Fatalf("rotateRealityShortIDs: %v", err)
	}
	wantActive := []string{"2222222222222222", "bbbb", "cccccc", "1111111111111111"}
	wantRetiring := []string{"dddddddd", "aa"}
	if !slices.Equal(rotation.ActiveIDs, wantActive) || !slices.Equal(rotation.RetiringIDs, wantRetiring) {
		t.Fatalf("rotation = active %v, retiring %v; want active %v, retiring %v", rotation.ActiveIDs, rotation.RetiringIDs, wantActive, wantRetiring)
	}
	if rotation.NextCursor != 1 {
		t.Fatalf("next cursor = %d, want 1", rotation.NextCursor)
	}
}

func TestRetireRealityShortIDs(t *testing.T) {
	rotation, err := rotateRealityShortIDs(
		realityRotationStream,
		4,
		0,
		2,
		sequenceShortIDGenerator("1111111111111111", "2222222222222222"),
	)
	if err != nil {
		t.Fatalf("rotateRealityShortIDs: %v", err)
	}

	cleaned, retired, err := retireRealityShortIDs(rotation.StreamSettings, rotation.ActiveCount)
	if err != nil {
		t.Fatalf("retireRealityShortIDs: %v", err)
	}
	if !slices.Equal(retired, rotation.RetiringIDs) {
		t.Fatalf("retired IDs = %v, want %v", retired, rotation.RetiringIDs)
	}
	if got := shortIDsFromStream(t, cleaned); !slices.Equal(got, rotation.ActiveIDs) {
		t.Fatalf("cleaned IDs = %v, want %v", got, rotation.ActiveIDs)
	}
}

func TestRotateRealityShortIDs_RejectsPendingRetirement(t *testing.T) {
	withRetiring := `{"security":"reality","realitySettings":{"shortIds":["aa","bbbb"]}}`
	_, err := rotateRealityShortIDs(withRetiring, 1, 0, 1, sequenceShortIDGenerator("1111111111111111"))
	if !errors.Is(err, errRealityShortIDsRetiring) {
		t.Fatalf("error = %v, want errRealityShortIDsRetiring", err)
	}
}

func TestRotateRealityShortIDs_RetriesDuplicateCandidate(t *testing.T) {
	rotation, err := rotateRealityShortIDs(
		realityRotationStream,
		4,
		0,
		1,
		sequenceShortIDGenerator("aa", "1111111111111111"),
	)
	if err != nil {
		t.Fatalf("rotateRealityShortIDs: %v", err)
	}
	if rotation.ActiveIDs[0] != "1111111111111111" {
		t.Fatalf("replacement = %q, want unique candidate", rotation.ActiveIDs[0])
	}
}

func TestRotateRealityShortIDs_PreservesOtherRealitySettings(t *testing.T) {
	rotation, err := rotateRealityShortIDs(
		realityRotationStream,
		4,
		0,
		1,
		sequenceShortIDGenerator("1111111111111111"),
	)
	if err != nil {
		t.Fatalf("rotateRealityShortIDs: %v", err)
	}
	var stream map[string]any
	if err := json.Unmarshal([]byte(rotation.StreamSettings), &stream); err != nil {
		t.Fatalf("decode rotated stream: %v", err)
	}
	reality := stream["realitySettings"].(map[string]any)
	if reality["target"] != "example.com:443" || stream["network"] != "tcp" {
		t.Fatalf("unrelated stream settings were changed: %v", stream)
	}
}

func TestRotateRealityShortIDs_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		stream string
	}{
		{name: "invalid JSON", stream: `{`},
		{name: "not REALITY", stream: `{"security":"tls","realitySettings":{"shortIds":["aa"]}}`},
		{name: "missing settings", stream: `{"security":"reality"}`},
		{name: "odd short ID", stream: `{"security":"reality","realitySettings":{"shortIds":["abc"]}}`},
		{name: "non-string short ID", stream: `{"security":"reality","realitySettings":{"shortIds":[42]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := rotateRealityShortIDs(tt.stream, 0, 0, 0, nil); err == nil {
				t.Fatal("rotateRealityShortIDs returned nil error")
			}
		})
	}
}
