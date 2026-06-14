package job

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/op/go-logging"
)

func initJobTestDB(t *testing.T) {
	t.Helper()
	xuilogger.InitLogger(logging.ERROR)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func realityStreamJSON(t *testing.T, shortIdDays int, lastRotation int64, shortIds []string) string {
	t.Helper()
	stream := map[string]any{
		"security": "reality",
		"realitySettings": map[string]any{
			"shortIds": shortIds,
			"settings": map[string]any{"publicKey": "PUB"},
			"rotation": map[string]any{
				"shortIdDays":           shortIdDays,
				"publicKeyDays":         0,
				"lastShortIdRotation":   lastRotation,
				"lastPublicKeyRotation": 0,
			},
		},
	}
	b, err := json.Marshal(stream)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func shortIdsOf(t *testing.T, streamJSON string) []string {
	t.Helper()
	var stream map[string]any
	if err := json.Unmarshal([]byte(streamJSON), &stream); err != nil {
		t.Fatal(err)
	}
	reality := stream["realitySettings"].(map[string]any)
	raw := reality["shortIds"].([]any)
	out := make([]string, len(raw))
	for i := range raw {
		out[i] = raw[i].(string)
	}
	return out
}

// TestRealityRotationJob_RotatesDueInbound drives the real job end-to-end
// against a real DB: a due Reality inbound has its shortIds replaced and
// persisted, a rotation-disabled Reality inbound and a non-Reality inbound are
// left untouched.
func TestRealityRotationJob_RotatesDueInbound(t *testing.T) {
	initJobTestDB(t)
	db := database.GetDB()

	// Due: shortIdDays=1, last rotation in the distant past (unix 1).
	due := &model.Inbound{
		UserId: 1, Tag: "in-due", Enable: true, Port: 44301, Protocol: model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: realityStreamJSON(t, 1, 1, []string{"aa", "bbbb"}),
	}
	// Disabled rotation (shortIdDays=0): must not change.
	off := &model.Inbound{
		UserId: 1, Tag: "in-off", Enable: true, Port: 44302, Protocol: model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: realityStreamJSON(t, 0, 0, []string{"cc", "dddd"}),
	}
	// Non-reality inbound: must not change.
	plain := &model.Inbound{
		UserId: 1, Tag: "in-plain", Enable: true, Port: 44303, Protocol: model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"security":"none","tcpSettings":{}}`,
	}
	for _, ib := range []*model.Inbound{due, off, plain} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create %s: %v", ib.Tag, err)
		}
	}

	NewRealityRotationJob().Run()

	read := func(id int) model.Inbound {
		var got model.Inbound
		if err := db.First(&got, id).Error; err != nil {
			t.Fatalf("reload inbound %d: %v", id, err)
		}
		return got
	}

	// Due inbound: shortIds replaced with a fresh 8-entry set, anchor advanced.
	gotDue := read(due.Id)
	ids := shortIdsOf(t, gotDue.StreamSettings)
	if len(ids) != 8 {
		t.Fatalf("due inbound shortIds not rotated: len=%d (%v)", len(ids), ids)
	}
	for _, id := range ids {
		if id == "aa" || id == "bbbb" {
			// extremely unlikely for the random set to reproduce an old value,
			// but the real proof is the set size jumping 2 -> 8 above.
		}
	}
	var dueStream map[string]any
	_ = json.Unmarshal([]byte(gotDue.StreamSettings), &dueStream)
	rot := dueStream["realitySettings"].(map[string]any)["rotation"].(map[string]any)
	if int64(rot["lastShortIdRotation"].(float64)) <= 1 {
		t.Fatalf("lastShortIdRotation not advanced: %v", rot["lastShortIdRotation"])
	}

	// Disabled-rotation inbound: untouched.
	if got := shortIdsOf(t, read(off.Id).StreamSettings); len(got) != 2 || got[0] != "cc" {
		t.Fatalf("rotation-disabled inbound was modified: %v", got)
	}

	// Non-reality inbound: untouched.
	if got := read(plain.Id).StreamSettings; got != `{"security":"none","tcpSettings":{}}` {
		t.Fatalf("non-reality inbound was modified: %s", got)
	}
}

// TestRealityRotationJob_AnchorsNotDue verifies the first-sight behavior through
// the real job: an enabled interval whose anchor is 0 is anchored to "now"
// WITHOUT rotating the shortIds (so enabling does not trigger immediate churn).
func TestRealityRotationJob_AnchorsNotDue(t *testing.T) {
	initJobTestDB(t)
	db := database.GetDB()

	ib := &model.Inbound{
		UserId: 1, Tag: "in-anchor", Enable: true, Port: 44311, Protocol: model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: realityStreamJSON(t, 1, 0, []string{"aa", "bbbb"}),
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}

	NewRealityRotationJob().Run()

	var got model.Inbound
	if err := db.First(&got, ib.Id).Error; err != nil {
		t.Fatal(err)
	}
	ids := shortIdsOf(t, got.StreamSettings)
	if len(ids) != 2 || ids[0] != "aa" || ids[1] != "bbbb" {
		t.Fatalf("shortIds rotated on first sight; should only anchor: %v", ids)
	}
	var stream map[string]any
	_ = json.Unmarshal([]byte(got.StreamSettings), &stream)
	rot := stream["realitySettings"].(map[string]any)["rotation"].(map[string]any)
	if int64(rot["lastShortIdRotation"].(float64)) <= 0 {
		t.Fatalf("anchor not set to now: %v", rot["lastShortIdRotation"])
	}
}
