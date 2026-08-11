package tgbot

import (
	"os"
	"strings"
	"testing"
)

func TestUpdateNumericInput(t *testing.T) {
	tests := []struct {
		name       string
		value, key int
		want       int
	}{
		{name: "append digit", value: 12, key: 3, want: 123},
		{name: "append zero", value: 12, key: 0, want: 120},
		{name: "backspace", value: 123, key: -1, want: 12},
		{name: "backspace zero", value: 0, key: -1, want: 0},
		{name: "clear", value: 123, key: -2, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateNumericInput(tt.value, tt.key); got != tt.want {
				t.Fatalf("updateNumericInput(%d, %d) = %d, want %d", tt.value, tt.key, got, tt.want)
			}
		})
	}
}

func TestNumericInputTransitionIsUsedByEveryKeypad(t *testing.T) {
	source, err := os.ReadFile("tgbot_router.go")
	if err != nil {
		t.Fatalf("read tgbot_router.go: %v", err)
	}
	if got := strings.Count(string(source), "updateNumericInput("); got != 6 {
		t.Fatalf("numeric keypad transition call sites = %d, want 6", got)
	}
}
