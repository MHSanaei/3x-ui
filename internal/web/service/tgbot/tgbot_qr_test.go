package tgbot

import "testing"

// The label is what a customer picks from, so it comes from the link's own
// remark — the same name their client app will show for that server.
func TestLinkLabel(t *testing.T) {
	tests := []struct {
		name  string
		link  string
		index int
		want  string
	}{
		{"remark fragment", "vless://uuid@host:443?type=tcp#Germany-Reality", 0, "1. Germany-Reality"},
		{"percent-encoded remark", "vless://uuid@host:443#Tokyo%20Node", 1, "2. Tokyo Node"},
		{"no fragment falls back to protocol", "trojan://pass@host:443", 2, "3. TROJAN"},
		{"empty fragment falls back", "vmess://payload#", 3, "4. VMESS"},
		{"blank fragment falls back", "ss://payload#%20", 4, "5. SS"},
		{"not a url at all", "garbage", 5, "Link 6"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := linkLabel(tc.link, tc.index); got != tc.want {
				t.Fatalf("linkLabel(%q, %d) = %q, want %q", tc.link, tc.index, got, tc.want)
			}
		})
	}
}

// A malformed callback must not be able to steer a QR at an arbitrary index.
func TestSplitQRTarget(t *testing.T) {
	tests := []struct {
		arg       string
		wantEmail string
		wantIndex int
		wantOK    bool
	}{
		{"alice@x 0", "alice@x", 0, true},
		{"alice@x 12", "alice@x", 12, true},
		{"  alice@x 3  ", "alice@x", 3, true},
		{"alice@x", "", 0, false},
		{"alice@x abc", "", 0, false},
		{"alice@x -1", "", 0, false},
		{" 4", "", 0, false},
		{"", "", 0, false},
	}

	for _, tc := range tests {
		email, index, ok := splitEmailIndexTarget(tc.arg)
		if email != tc.wantEmail || index != tc.wantIndex || ok != tc.wantOK {
			t.Errorf("splitEmailIndexTarget(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.arg, email, index, ok, tc.wantEmail, tc.wantIndex, tc.wantOK)
		}
	}
}
