package outbound

import "testing"

func TestParseCloudflareTrace(t *testing.T) {
	values := parseCloudflareTrace("ip=104.28.1.2\nloc=NL\nwarp=on\n")

	if values["ip"] != "104.28.1.2" {
		t.Fatalf("ip = %q", values["ip"])
	}
	if values["loc"] != "NL" {
		t.Fatalf("loc = %q", values["loc"])
	}
	if values["warp"] != "on" {
		t.Fatalf("warp = %q", values["warp"])
	}
}
