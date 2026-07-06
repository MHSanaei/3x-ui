package service

import "testing"

func TestPgRestoreReadFailureError(t *testing.T) {
	cases := []struct {
		name         string
		probeOutput  string
		localVersion string
		want         string
	}{
		{
			name:         "dump from postgres 17 on older client",
			probeOutput:  "pg_restore: error: unsupported version (1.16) in file header",
			localVersion: "16.4",
			want:         "This backup was created by pg_dump from PostgreSQL 17 or newer, but the server's pg_restore is version 16.4 and cannot read it; run 'x-ui pgclient 17' on the server (or upgrade the postgresql-client package to version 17 or newer), then retry the import",
		},
		{
			name:         "dump from postgres 16 on older client",
			probeOutput:  "pg_restore: error: unsupported version (1.15) in file header",
			localVersion: "15.8",
			want:         "This backup was created by pg_dump from PostgreSQL 16 or newer, but the server's pg_restore is version 15.8 and cannot read it; run 'x-ui pgclient 16' on the server (or upgrade the postgresql-client package to version 16 or newer), then retry the import",
		},
		{
			name:         "archive version newer than any known mapping",
			probeOutput:  "pg_restore: error: unsupported version (1.17) in file header",
			localVersion: "17.2",
			want:         "This backup was created by a newer pg_dump than the server's pg_restore (version 17.2) can read; upgrade the postgresql-client package and retry the import",
		},
		{
			name:         "local version could not be determined",
			probeOutput:  "pg_restore: error: unsupported version (1.16) in file header",
			localVersion: "",
			want:         "This backup was created by pg_dump from PostgreSQL 17 or newer, but the server's pg_restore is version unknown and cannot read it; run 'x-ui pgclient 17' on the server (or upgrade the postgresql-client package to version 17 or newer), then retry the import",
		},
		{
			name:         "unrelated read failure passes through",
			probeOutput:  "pg_restore: error: could not read from input file: end of file",
			localVersion: "16.4",
			want:         "pg_restore cannot read this dump file: pg_restore: error: could not read from input file: end of file",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := pgRestoreReadFailureError(tc.probeOutput, tc.localVersion)
			if err == nil {
				t.Fatal("pgRestoreReadFailureError returned nil, want error")
			}
			if err.Error() != tc.want {
				t.Errorf("pgRestoreReadFailureError(%q, %q) = %q, want %q", tc.probeOutput, tc.localVersion, err.Error(), tc.want)
			}
		})
	}
}

func TestParsePgToolVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "pg_restore (PostgreSQL) 17.2\n", "17.2"},
		{"debian packaging suffix", "pg_restore (PostgreSQL) 16.10 (Debian 16.10-1.pgdg120+1)\n", "16.10"},
		{"three component version", "pg_restore (PostgreSQL) 9.6.24\n", "9.6.24"},
		{"no version present", "pg_restore malfunction\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePgToolVersion(tc.in)
			if got != tc.want {
				t.Errorf("parsePgToolVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
