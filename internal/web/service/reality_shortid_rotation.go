package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

var errRealityShortIDsRetiring = errors.New("REALITY short ID retirement is still pending")

const maxRealityShortIDGenerationAttempts = 128

type realityShortIDGenerator func() (string, error)

type realityShortIDRotation struct {
	StreamSettings string
	ActiveCount    int
	NextCursor     int
	ActiveIDs      []string
	RetiringIDs    []string
}

// newRealityShortID returns the maximum-size short ID accepted by Xray:
// eight cryptographically random bytes encoded as 16 lowercase hex digits.
func newRealityShortID() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate REALITY short ID: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// The active prefix is publishable; replaced IDs move to the grace-only suffix.
// rotateCount == 0 replaces every active ID.
func rotateRealityShortIDs(
	streamSettings string,
	activeCount int,
	cursor int,
	rotateCount int,
	generate realityShortIDGenerator,
) (*realityShortIDRotation, error) {
	stream, reality, shortIDs, err := parseRealityShortIDs(streamSettings)
	if err != nil {
		return nil, err
	}
	if len(shortIDs) == 0 {
		return nil, errors.New("REALITY shortIds must contain at least one entry")
	}
	if activeCount <= 0 {
		activeCount = len(shortIDs)
	}
	if activeCount > len(shortIDs) {
		return nil, fmt.Errorf("REALITY active short ID count %d exceeds list length %d", activeCount, len(shortIDs))
	}
	if activeCount < len(shortIDs) {
		return nil, errRealityShortIDsRetiring
	}
	if rotateCount < 0 || rotateCount > activeCount {
		return nil, fmt.Errorf("REALITY rotation count %d must be between 0 and %d", rotateCount, activeCount)
	}
	if rotateCount == 0 {
		rotateCount = activeCount
	}
	if generate == nil {
		generate = newRealityShortID
	}

	active := append([]string(nil), shortIDs[:activeCount]...)
	retiring := make([]string, 0, rotateCount)
	used := make(map[string]struct{}, len(shortIDs)+rotateCount)
	for _, id := range shortIDs {
		used[id] = struct{}{}
	}

	cursor = normalizeRealityShortIDCursor(cursor, activeCount)
	for offset := range rotateCount {
		index := (cursor + offset) % activeCount
		retiring = append(retiring, active[index])

		var replacement string
		for attempt := 0; attempt < maxRealityShortIDGenerationAttempts; attempt++ {
			candidate, genErr := generate()
			if genErr != nil {
				return nil, genErr
			}
			if !validRealityShortID(candidate) {
				return nil, fmt.Errorf("generated invalid REALITY short ID %q", candidate)
			}
			if _, exists := used[candidate]; exists {
				continue
			}
			replacement = candidate
			used[candidate] = struct{}{}
			break
		}
		if replacement == "" {
			return nil, errors.New("could not generate a unique REALITY short ID")
		}
		active[index] = replacement
	}

	accepted := append(append([]string(nil), active...), retiring...)
	reality["shortIds"] = accepted
	encoded, err := json.MarshalIndent(stream, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode REALITY stream settings: %w", err)
	}

	return &realityShortIDRotation{
		StreamSettings: string(encoded),
		ActiveCount:    len(active),
		NextCursor:     (cursor + rotateCount) % activeCount,
		ActiveIDs:      active,
		RetiringIDs:    retiring,
	}, nil
}

// retireRealityShortIDs removes the suffix retained for the grace period.
func retireRealityShortIDs(streamSettings string, activeCount int) (string, []string, error) {
	stream, reality, shortIDs, err := parseRealityShortIDs(streamSettings)
	if err != nil {
		return "", nil, err
	}
	if activeCount <= 0 || activeCount > len(shortIDs) {
		return "", nil, fmt.Errorf("REALITY active short ID count %d is invalid for list length %d", activeCount, len(shortIDs))
	}

	retired := append([]string(nil), shortIDs[activeCount:]...)
	reality["shortIds"] = append([]string(nil), shortIDs[:activeCount]...)
	encoded, err := json.MarshalIndent(stream, "", "  ")
	if err != nil {
		return "", nil, fmt.Errorf("encode REALITY stream settings: %w", err)
	}
	return string(encoded), retired, nil
}

func parseRealityShortIDs(streamSettings string) (map[string]any, map[string]any, []string, error) {
	var stream map[string]any
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return nil, nil, nil, fmt.Errorf("parse stream settings: %w", err)
	}
	if security, _ := stream["security"].(string); security != "reality" {
		return nil, nil, nil, errors.New("inbound does not use REALITY security")
	}
	reality, ok := stream["realitySettings"].(map[string]any)
	if !ok || reality == nil {
		return nil, nil, nil, errors.New("REALITY settings are missing")
	}
	rawIDs, ok := reality["shortIds"].([]any)
	if !ok {
		return nil, nil, nil, errors.New("REALITY shortIds must be an array")
	}
	shortIDs := make([]string, len(rawIDs))
	for i, raw := range rawIDs {
		id, ok := raw.(string)
		if !ok {
			return nil, nil, nil, fmt.Errorf("REALITY shortIds[%d] must be a string", i)
		}
		if !validRealityShortID(id) {
			return nil, nil, nil, fmt.Errorf("REALITY shortIds[%d] has invalid format", i)
		}
		shortIDs[i] = id
	}
	return stream, reality, shortIDs, nil
}

func normalizeRealityShortIDCursor(cursor, activeCount int) int {
	if activeCount <= 0 {
		return 0
	}
	cursor %= activeCount
	if cursor < 0 {
		cursor += activeCount
	}
	return cursor
}

func validRealityShortID(id string) bool {
	if len(id) > 16 || len(id)%2 != 0 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
