package amneziawg

import "testing"

func TestForwardedPortsInclude(t *testing.T) {
	cases := []struct {
		spec string
		port int
		want bool
	}{
		{"80,443", 80, true},
		{"80,443", 443, true},
		{"80,443", 8080, false},
		{"8000-8100", 8050, true},
		{"8000-8100", 7999, false},
		{"8000-8100", 8101, false},
		{"", 80, false},
		{"not-a-port", 80, false},
	}
	for _, c := range cases {
		if got := ForwardedPortsInclude(c.spec, c.port); got != c.want {
			t.Errorf("ForwardedPortsInclude(%q, %d) = %v, want %v", c.spec, c.port, got, c.want)
		}
	}
}
