package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// MASQUE is native Xray, so a node runs it, but a node panel older than the release
// that knows it would hand the clients' "password" to a core that reads "pass".
func TestCheckNodeCanHostProtocol_Masque(t *testing.T) {
	setupConflictDB(t)
	tests := []struct {
		version string
		wantErr string
	}{
		{"v3.9.0", "need v3.9.1 or newer"},
		{"v3.9.1", ""},
		{"dev+1d1128cf", ""},
	}
	for i, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			node := &model.Node{Name: "masque-node-" + tc.version, Address: "127.0.0.1", Port: 2096 + i, PanelVersion: tc.version}
			if err := database.GetDB().Create(node).Error; err != nil {
				t.Fatalf("create node: %v", err)
			}
			err := checkNodeCanHostProtocol(database.GetDB(), node.Id, model.MASQUE)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("node on %s refused MASQUE: %v", tc.version, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("node on %s: err = %v, want one containing %q", tc.version, err, tc.wantErr)
			}
		})
	}
}
