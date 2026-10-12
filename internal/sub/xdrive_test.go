package sub

import "testing"

// XDRIVE has no share-link format: a link would either drop the storage secrets both
// ends need or print them into every QR code, so only the JSON subscription carries it.
func TestGetLink_SkipsXdriveStreams(t *testing.T) {
	stream := `{"network":"xdrive","security":"none",
		"xdriveSettings":{"service":"Google Drive","remoteFolder":"xray-tunnel","secrets":["id","secret","token"]}}`
	inbound := flowTestInbound(stream, "none")

	if link := (&SubService{}).GetLink(inbound, "user"); link != "" {
		t.Fatalf("GetLink = %q, want no link for an XDRIVE stream", link)
	}
}
