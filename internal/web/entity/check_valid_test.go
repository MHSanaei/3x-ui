package entity

import (
	"strings"
	"testing"
)

func TestCheckValidSmtpFrom(t *testing.T) {
	base := func() *AllSetting {
		return &AllSetting{WebPort: 2053, SubPort: 2096}
	}

	for _, v := range []string{"", "panel@example.com"} {
		s := base()
		s.SmtpFrom = v
		if err := s.CheckValid(); err != nil {
			t.Errorf("CheckValid with smtpFrom=%q: unexpected error %v", v, err)
		}
	}

	for _, v := range []string{
		"not-an-address",
		"panel@example.com\r\nBcc: evil@example.com",
		"a@b\nSubject: injected",
	} {
		s := base()
		s.SmtpFrom = v
		if err := s.CheckValid(); err == nil {
			t.Errorf("CheckValid with smtpFrom=%q: want error, got nil", v)
		}
	}
}

func TestCheckValidWildcardListenPortConflict(t *testing.T) {
	s := &AllSetting{WebPort: 2053, SubPort: 2053, WebListen: "0.0.0.0", SubListen: ""}
	if err := s.CheckValid(); err == nil {
		t.Error("CheckValid must reject the same port bound on 0.0.0.0 and \"\" (both wildcard)")
	}

	ok := &AllSetting{WebPort: 2053, SubPort: 2053, WebListen: "127.0.0.1", SubListen: "192.168.1.1"}
	if err := ok.CheckValid(); err != nil {
		t.Errorf("distinct specific listens on the same port should be allowed: %v", err)
	}
}

// The allowlist and the trusted-proxy list share one validator, so this also
// pins that each list still reports its own message (#5378).
func TestCheckValidIPOrCIDRLists(t *testing.T) {
	base := func() *AllSetting {
		return &AllSetting{WebPort: 2053, SubPort: 2096}
	}

	for _, v := range []string{"", "203.0.113.10", "198.51.100.0/24", " 203.0.113.10 , 2001:db8::/32 ", "203.0.113.10,,"} {
		s := base()
		s.IpLimitAllowlist = v
		if err := s.CheckValid(); err != nil {
			t.Errorf("ipLimitAllowlist=%q: unexpected error %v", v, err)
		}
	}

	for _, v := range []string{"nonsense", "203.0.113.10/33", "203.0.113.10, oops"} {
		s := base()
		s.IpLimitAllowlist = v
		err := s.CheckValid()
		if err == nil {
			t.Errorf("ipLimitAllowlist=%q: want error, got nil", v)
			continue
		}
		if !strings.Contains(err.Error(), "IP limit allowlist entry is not valid:") {
			t.Errorf("ipLimitAllowlist=%q: error %q does not name the setting", v, err)
		}
	}

	s := base()
	s.TrustedProxyCIDRs = "127.0.0.1/32, bogus"
	err := s.CheckValid()
	if err == nil || !strings.Contains(err.Error(), "trusted proxy CIDR is not valid: bogus") {
		t.Errorf("trustedProxyCIDRs error = %v, want it to name the trusted-proxy list and the bad entry", err)
	}
}
