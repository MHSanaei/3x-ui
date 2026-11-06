package service

import "testing"

func TestGetListenPort(t *testing.T) {
	cases := []struct {
		name    string
		xuiPort string
		want    int
	}{
		// 2053 is the default webPort on a fresh database.
		{"XUI_PORT unset", "", 2053},
		{"valid XUI_PORT", "8443", 8443},
		{"XUI_PORT not a number", "abc", 2053},
		{"XUI_PORT out of range", "70000", 2053},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			t.Setenv("XUI_PORT", tc.xuiPort)
			got, err := (&SettingService{}).GetListenPort()
			if err != nil {
				t.Fatalf("GetListenPort: %v", err)
			}
			if got != tc.want {
				t.Fatalf("GetListenPort() = %d, want %d", got, tc.want)
			}
		})
	}
}
