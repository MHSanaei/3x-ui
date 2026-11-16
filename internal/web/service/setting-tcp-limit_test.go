package service

import "testing"

func TestTCPPerIPConnectionLimitDefaultsAndValidation(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.TcpPerIpConnectionLimit != 0 {
		t.Fatalf("default limit = %d, want disabled", settings.TcpPerIpConnectionLimit)
	}
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatalf("saving disabled limit: %v", err)
	}
	for _, limit := range []int{-1, 1000001} {
		settings.TcpPerIpConnectionLimit = limit
		if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil {
			t.Fatalf("invalid limit %d accepted", limit)
		}
		if got, err := s.GetTCPPerIPConnectionLimit(); err != nil || got != 0 {
			t.Fatalf("invalid save changed limit to %d, error %v", got, err)
		}
	}
}
