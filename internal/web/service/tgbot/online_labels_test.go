package tgbot

import "testing"

func TestOnlineButtonLabel(t *testing.T) {
	remarks := map[string]string{"in-443": "Germany", "in-8443": "", "in-80": "Finland"}
	cases := []struct {
		name     string
		tags     []string
		fallback string
		want     string
	}{
		{"single inbound", []string{"in-443"}, "", "alice - Germany"},
		{"multiple inbounds all listed", []string{"in-443", "in-80"}, "", "alice - Germany, Finland"},
		{"tag without remark falls back to tag", []string{"in-8443"}, "", "alice - in-8443"},
		{"duplicate names collapse", []string{"in-443", "in-443"}, "", "alice - Germany"},
		{"no attribution uses legacy remark", nil, "Old", "alice - Old"},
		{"no attribution and no remark is just the email", nil, "", "alice"},
		{"attribution wins over legacy remark", []string{"in-80"}, "Old", "alice - Finland"},
	}
	for _, c := range cases {
		if got := onlineButtonLabel("alice", c.tags, remarks, c.fallback); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
