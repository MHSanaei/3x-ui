package tgbot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTgbotLocalesQrKeyValid(t *testing.T) {
	dir := filepath.Join("..", "..", "translation")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Errorf("%s read: %v", e.Name(), err)
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Errorf("%s invalid JSON: %v", e.Name(), err)
			continue
		}
		answers, ok := raw["tgbot"].(map[string]any)["answers"].(map[string]any)
		if !ok {
			t.Errorf("%s no tgbot.answers", e.Name())
			continue
		}
		if _, ok := answers["qrCodeForClient"].(string); !ok {
			t.Errorf("%s missing qrCodeForClient", e.Name())
		}
	}
}
