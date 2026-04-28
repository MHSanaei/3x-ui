package model

import "testing"

func TestIsHysteria(t *testing.T) {
	cases := []struct {
		in   Protocol
		want bool
	}{
		{Hysteria, true},
		{Hysteria2, true},
		{VLESS, false},
		{Shadowsocks, false},
		{Protocol(""), false},
		{Protocol("hysteria3"), false},
	}
	for _, c := range cases {
		if got := IsHysteria(c.in); got != c.want {
			t.Errorf("IsHysteria(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
