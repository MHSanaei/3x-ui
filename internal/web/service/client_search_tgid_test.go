package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientMatchesSearch_TgID(t *testing.T) {
	withTgID := ClientWithAttachments{
		ClientRecord: model.ClientRecord{Email: "alice@example.com", TgID: 759865},
	}
	withoutTgID := ClientWithAttachments{
		ClientRecord: model.ClientRecord{Email: "bob@example.com", TgID: 0},
	}

	tests := []struct {
		name   string
		client ClientWithAttachments
		needle string
		want   bool
	}{
		{
			name:   "matches full tgId",
			client: withTgID,
			needle: "759865",
			want:   true,
		},
		{
			name:   "matches tgId substring",
			client: withTgID,
			needle: "9865",
			want:   true,
		},
		{
			name:   "no match on unrelated numeric needle",
			client: withTgID,
			needle: "42",
			want:   false,
		},
		{
			name:   "needle '0' does not spuriously match a client without a tgId",
			client: withoutTgID,
			needle: "0",
			want:   false,
		},
		{
			name:   "still matches on the existing fields (no regression)",
			client: withTgID,
			needle: "alice",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clientMatchesSearch(tt.client, tt.needle); got != tt.want {
				t.Errorf("clientMatchesSearch(%+v, %q) = %v, want %v", tt.client.ClientRecord, tt.needle, got, tt.want)
			}
		})
	}
}
