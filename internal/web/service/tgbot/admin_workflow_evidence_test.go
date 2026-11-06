package tgbot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeBotScreenEvidence(t *testing.T, tb *Tgbot, chatID int64, name string) {
	t.Helper()
	dir := os.Getenv("XUI_TGBOT_SCREENSHOT_DIR")
	if dir == "" {
		return
	}
	sc, ok := tb.screens().get(chatID)
	if !ok {
		t.Fatal("missing screen evidence")
	}
	var buttons [][]string
	for _, row := range sc.rows {
		var labels []string
		for _, button := range row {
			labels = append(labels, button.Text)
		}
		buttons = append(buttons, labels)
	}
	data, err := json.MarshalIndent(struct {
		Body    string     `json:"body"`
		Buttons [][]string `json:"buttons"`
	}{sc.body, buttons}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
