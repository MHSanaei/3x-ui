package xray

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

func TestPersistConfigWritesCurrentSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	p := NewTestProcess(&Config{RouterConfig: json_util.RawMessage(`{"rules":[]}`)}, path)

	p.SetConfig(&Config{RouterConfig: json_util.RawMessage(`{"rules":[{"outboundTag":"warp"}]}`)})
	if err := p.PersistConfig(); err != nil {
		t.Fatalf("PersistConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"outboundTag": "warp"`) && !strings.Contains(string(data), `"outboundTag":"warp"`) {
		t.Fatalf("config file does not hold the new routing:\n%s", data)
	}
}
