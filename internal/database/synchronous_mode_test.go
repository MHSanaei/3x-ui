package database

import "testing"

func TestSqliteSynchronousDefaultsToNormal(t *testing.T) {
	t.Setenv("XUI_DB_SYNCHRONOUS", "")
	if got := sqliteSynchronous(); got != "NORMAL" {
		t.Fatalf("default synchronous = %q, want NORMAL", got)
	}
}

func TestSqliteSynchronousEnvOverride(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want string
	}{
		{"full", "FULL"},
		{"FULL", "FULL"},
		{"extra", "EXTRA"},
		{"off", "OFF"},
		{" natural ", "NORMAL"},
	} {
		t.Setenv("XUI_DB_SYNCHRONOUS", tc.env)
		if got := sqliteSynchronous(); got != tc.want {
			t.Fatalf("XUI_DB_SYNCHRONOUS=%q: synchronous = %q, want %q", tc.env, got, tc.want)
		}
	}
}
