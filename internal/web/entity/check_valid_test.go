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
