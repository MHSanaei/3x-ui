package entity

import "testing"

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
