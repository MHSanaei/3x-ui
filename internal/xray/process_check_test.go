package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// A kernel-TUN WireGuard outbound in `run -test` adds a second wg device and IPv6 policy
// rules to the host, and the check exits without removing the rules.
func TestCheckConfigKeepsWireGuardOffKernelTun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}
	dir := t.TempDir()
	checked := filepath.Join(dir, "checked.json")
	core := filepath.Join(dir, "xray")
	script := "#!/bin/sh\n[ \"$1 $2 $3\" = 'run -test -c' ] || exit 99\nexec cp \"$4\" '" + checked + "'\n"
	if err := os.WriteFile(core, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{OutboundConfigs: json_util.RawMessage(
		`[{"protocol":"wireguard","tag":"warp","settings":{"noKernelTun":false,"mtu":1420}}]`)}
	if err := CheckConfig(core, cfg); err != nil {
		t.Fatalf("CheckConfig: %v", err)
	}

	data, err := os.ReadFile(checked)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Outbounds []struct {
			Settings map[string]any `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("checked config: %v\n%s", err, data)
	}
	if len(got.Outbounds) != 1 || got.Outbounds[0].Settings["noKernelTun"] != true {
		t.Fatalf("checked outbounds = %+v, want the wireguard outbound on noKernelTun", got.Outbounds)
	}
}
