package ldaputil

import "testing"

func TestTLSConfig_InsecureSkipVerifyPropagates(t *testing.T) {
	cases := []struct {
		name string
		skip bool
		want bool
	}{
		{"default verifies", false, false},
		{"skip flows through", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tlsConfig(Config{InsecureSkipVerify: c.skip})
			if got.InsecureSkipVerify != c.want {
				t.Fatalf("InsecureSkipVerify = %v, want %v", got.InsecureSkipVerify, c.want)
			}
		})
	}
}
