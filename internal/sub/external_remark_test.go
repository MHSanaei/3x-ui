package sub

import (
	"strings"
	"testing"
)

// #6575: a trailing ?serverDescription=<base64> must stay literal in the
// fragment so Happ renders its subtitle; only the display name is escaped.
func TestApplyRemarkKeepsServerDescription(t *testing.T) {
	link := "vless://00000000-0000-0000-0000-000000000000@example.com:443?type=tcp&security=reality&pbk=XXX&fp=chrome&sni=example.org&sid=00&flow=xtls-rprx-vision&encryption=none"
	remark := "🇵🇱 Warsaw ⚡️?serverDescription=0JTQu9GPIExURSAo0LHQtdC70YvQtSDRgdC/0LjRgdC60Lgp"

	out := applyRemarkToLink(link, remark)
	frag := out[strings.IndexByte(out, '#')+1:]
	if !strings.Contains(frag, "?serverDescription=") {
		t.Fatalf("serverDescription escaped: %s", out)
	}
	if strings.Contains(frag, "%3F") || strings.Contains(frag, "%2F") {
		t.Fatalf("fragment over-escaped: %s", out)
	}
	tail := frag[strings.Index(frag, "?serverDescription=")+len("?serverDescription="):]
	if strings.ContainsAny(tail, " \r\n\t#&") {
		t.Fatalf("tail not clean base64: %q", tail)
	}
	if !strings.HasPrefix(out, link+"#") {
		t.Fatalf("link body altered: %s", out)
	}
}

func TestApplyRemarkMalformedServerDescriptionFallsBack(t *testing.T) {
	link := "vless://uuid@example.com:443?security=reality#old"
	out := applyRemarkToLink(link, "name?serverDescription=not base64!!")
	if strings.Contains(out, "?serverDescription=") {
		t.Fatalf("malformed tail kept literal: %s", out)
	}
	if !strings.HasPrefix(out, link[:strings.IndexByte(link, '#')]+"#") {
		t.Fatalf("link body altered: %s", out)
	}
}
