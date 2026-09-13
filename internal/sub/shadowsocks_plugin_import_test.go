package sub

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
)

// The panel exports shadowsocks tcp/http obfuscation as the SIP002 plugin, so
// importing that same link has to rebuild the header it stands for.
func TestShadowsocksHTTPObfsSurvivesExportImport(t *testing.T) {
	in := &model.Inbound{
		Id: 940001, Listen: "203.0.113.1", Port: 8388, Protocol: model.Shadowsocks,
		Settings:       `{"method":"aes-256-gcm","password":"serverpass","clients":[{"email":"user","password":"clientpass"}]}`,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"http","request":{"path":["/"],"headers":{"Host":["obfs.example.com"]}}}}}`,
	}
	exported := (&SubService{}).genShadowsocksLink(in, "user")
	if !strings.Contains(exported, "plugin=obfs-local%3Bobfs%3Dhttp") {
		t.Fatalf("export did not use the SIP002 plugin form: %q", exported)
	}

	parsed, err := link.ParseLink(exported)
	if err != nil {
		t.Fatalf("ParseLink(%q): %v", exported, err)
	}
	streamJSON, err := json.Marshal(parsed.Outbound["streamSettings"])
	if err != nil {
		t.Fatalf("marshal stream: %v", err)
	}
	var stream map[string]any
	if err := json.Unmarshal(streamJSON, &stream); err != nil {
		t.Fatalf("stream json: %v", err)
	}

	tcp, _ := stream["tcpSettings"].(map[string]any)
	header, _ := tcp["header"].(map[string]any)
	if header == nil || header["type"] != "http" {
		t.Fatalf("import dropped the tcp/http obfuscation: %s", streamJSON)
	}
	request, _ := header["request"].(map[string]any)
	headers, _ := request["headers"].(map[string]any)
	hosts, _ := headers["Host"].([]any)
	if len(hosts) == 0 || hosts[0] != "obfs.example.com" {
		t.Fatalf("import dropped the obfs host: %s", streamJSON)
	}
}
