package service

import (
	"strings"
	"testing"
)

func TestValidateXhttpDownload(t *testing.T) {
	const valid = `{"network":"xhttp","xhttpSettings":{"mode":"packet-up","extra":{"downloadSettings":{"network":"xhttp","address":"down.test","port":443,"security":"reality","realitySettings":{"password":"AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE","shortId":"abcd"}}}}}`
	for _, tc := range []struct{ from, to, field string }{
		{"packet-up", "packet-up", ""},
		{"packet-up", "stream-one", "stream-one"},
		{`"network":"xhttp","address"`, `"network":"tcp","address"`, "network"},
		{"AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE", "abc", "password"},
		{"abcd", "f", "shortId"},
	} {
		err := validateXhttpDownload(strings.Replace(valid, tc.from, tc.to, 1))
		if tc.field == "" && err != nil || tc.field != "" && (err == nil || !strings.Contains(err.Error(), tc.field)) {
			t.Fatalf("%s: %v", tc.to, err)
		}
	}
}
