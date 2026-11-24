package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestValidateXhttpDownload(t *testing.T) {
	const valid = `{"network":"xhttp","xhttpSettings":{"mode":"packet-up","extra":{"downloadSettings":{"network":"xhttp","address":"down.test","port":443,"security":"reality","realitySettings":{"password":"AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE","shortId":"abcd"}}}}}`
	for _, tc := range []struct{ from, to, field string }{
		{"packet-up", "packet-up", ""},
		{"packet-up", "stream-one", "stream-one"},
		{`"network":"xhttp","address"`, `"network":"tcp","address"`, "network"},
		{"AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE", "abc", "password"},
		{"abcd", "f", "shortId"},
		{`"shortId":"abcd"`, `"shortId":"abcd","fingerprint":"Chrome"`, ""},
		{`"shortId":"abcd"`, `"shortId":"abcd","fingerprint":"chorme"`, "fingerprint"},
		{`"shortId":"abcd"`, `"shortId":"abcd","fingerprint":"unsafe"`, "fingerprint"},
		{`"security":"reality","realitySettings"`, `"security":"tls","tlsSettings":{"fingerprint":"chorme"},"realitySettings"`, "fingerprint"},
		{`"security":"reality","realitySettings"`, `"security":"tls","tlsSettings":{"fingerprint":"firefox"},"realitySettings"`, ""},
	} {
		err := validateXhttpDownload(strings.Replace(valid, tc.from, tc.to, 1))
		if tc.field == "" && err != nil || tc.field != "" && (err == nil || !strings.Contains(err.Error(), tc.field)) {
			t.Fatalf("%s: %v", tc.to, err)
		}
	}
}

// The listener matches a download GET against its own session and padding rules, so a leg
// saved before the inbound's rules changed must not keep the old ones.
func TestNormalizeStreamSettingsSyncsXhttpDownloadRequestShape(t *testing.T) {
	inbound := &model.Inbound{Protocol: model.VLESS, StreamSettings: `{"network":"xhttp","xhttpSettings":{
		"path":"/up","sessionIDPlacement":"query","sessionIDKey":"sid","xPaddingBytes":"100-200",
		"downloadSettings":{"network":"xhttp","address":"down.test","port":443,"security":"tls",
			"xhttpSettings":{"path":"/down","host":"cdn.test","sessionIDPlacement":"path","seqKey":"stale"}}}}`}
	(&InboundService{}).normalizeStreamSettings(inbound)

	var stream struct {
		XhttpSettings struct {
			DownloadSettings struct {
				XhttpSettings map[string]any `json:"xhttpSettings"`
			} `json:"downloadSettings"`
		} `json:"xhttpSettings"`
	}
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"path": "/down", "host": "cdn.test",
		"sessionIDPlacement": "query", "sessionIDKey": "sid", "xPaddingBytes": "100-200",
	}
	if got := stream.XhttpSettings.DownloadSettings.XhttpSettings; !reflect.DeepEqual(got, want) {
		t.Fatalf("download leg xhttpSettings = %v, want %v", got, want)
	}
}
