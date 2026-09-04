package psiphon

import (
	"os"
	"strings"
	"testing"
)

func writeNotices(t *testing.T, lines ...string) {
	t.Helper()
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(NoticesPath(), []byte(content), 0o600); err != nil {
		t.Fatalf("writing notices fixture: %v", err)
	}
}

func TestCurrentTunnelMissingFile(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	status, err := CurrentTunnel()
	if err != nil {
		t.Fatalf("CurrentTunnel with no log file: %v", err)
	}
	if status != (TunnelStatus{}) {
		t.Errorf("CurrentTunnel with no log file = %+v, want a zero TunnelStatus", status)
	}
}

func TestCurrentTunnelParsesLatestOfEach(t *testing.T) {
	writeNotices(t,
		`{"noticeType":"ClientRegion","data":{"region":"LV"}}`,
		`{"noticeType":"Tunnels","data":{"count":0}}`,
		`{"noticeType":"Tunnels","data":{"count":1}}`,
		`{"noticeType":"ConnectedServerRegion","data":{"serverRegion":"BE"}}`,
		`{"noticeType":"ConnectedServerRegion","data":{"serverRegion":"JP"}}`,
	)
	status, err := CurrentTunnel()
	if err != nil {
		t.Fatalf("CurrentTunnel: %v", err)
	}
	want := TunnelStatus{Connected: true, TunnelCount: 1, ServerRegion: "JP", ClientRegion: "LV"}
	if status != want {
		t.Errorf("CurrentTunnel = %+v, want %+v", status, want)
	}
}

func TestCurrentTunnelZeroCountMeansNotConnected(t *testing.T) {
	writeNotices(t, `{"noticeType":"Tunnels","data":{"count":0}}`)
	status, err := CurrentTunnel()
	if err != nil {
		t.Fatalf("CurrentTunnel: %v", err)
	}
	if status.Connected {
		t.Errorf("CurrentTunnel.Connected = true for a Tunnels count of 0")
	}
}

func TestCurrentTunnelSkipsMalformedLines(t *testing.T) {
	writeNotices(t,
		`not even json`,
		`{"noticeType":"Tunnels","data":{"count":1}}`,
		``,
		`{this is not valid json either`,
	)
	status, err := CurrentTunnel()
	if err != nil {
		t.Fatalf("CurrentTunnel with malformed lines mixed in: %v", err)
	}
	if !status.Connected || status.TunnelCount != 1 {
		t.Errorf("CurrentTunnel = %+v, want the one well-formed notice still picked up", status)
	}
}

// The tail-scan optimization (added after the bot review flagged unbounded
// log growth) must not lose a notice that is still within the scanned tail.
func TestCurrentTunnelTailScanFindsRecentNotice(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	padding := strings.Repeat(`{"noticeType":"Ignored","data":{}}`+"\n", 20000)
	content := padding + `{"noticeType":"Tunnels","data":{"count":1}}` + "\n"
	if len(content)-len(`{"noticeType":"Tunnels","data":{"count":1}}`+"\n") < tailScanBytes {
		t.Fatalf("test fixture padding (%d bytes) does not exceed tailScanBytes (%d); grow padding", len(padding), tailScanBytes)
	}
	if err := os.WriteFile(NoticesPath(), []byte(content), 0o600); err != nil {
		t.Fatalf("writing notices fixture: %v", err)
	}

	status, err := CurrentTunnel()
	if err != nil {
		t.Fatalf("CurrentTunnel: %v", err)
	}
	if !status.Connected || status.TunnelCount != 1 {
		t.Errorf("CurrentTunnel over a >tailScanBytes log = %+v, want the trailing Tunnels notice picked up", status)
	}
}
