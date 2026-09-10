package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// A log path must never escape the log folder whatever case the key is written
// in: xray-core matches JSON keys onto its struct fields case-insensitively.
func TestResolveXrayLogPathsConfinesEveryKeyCase(t *testing.T) {
	folder := config.GetLogFolder()
	tests := []struct {
		name string
		in   string
		want map[string]any
	}{
		{
			name: "lowercase keys",
			in:   `{"access":"/tmp/pwn.log","error":"/tmp/pwn-err.log"}`,
			want: map[string]any{"access": filepath.Join(folder, "pwn.log"), "error": filepath.Join(folder, "pwn-err.log")},
		},
		{
			name: "capitalised keys",
			in:   `{"Access":"/tmp/pwn.log","Error":"/tmp/pwn-err.log"}`,
			want: map[string]any{"access": filepath.Join(folder, "pwn.log"), "error": filepath.Join(folder, "pwn-err.log")},
		},
		{
			name: "upper-case keys",
			in:   `{"ACCESS":"/tmp/pwn.log"}`,
			want: map[string]any{"access": filepath.Join(folder, "pwn.log")},
		},
		{
			name: "a variant cannot smuggle a path past a none",
			in:   `{"access":"none","Access":"/tmp/pwn.log"}`,
			want: map[string]any{"access": "none"},
		},
		{
			name: "already confined name is left alone",
			in:   `{"access":"none","error":"none","loglevel":"warning"}`,
			want: map[string]any{"access": "none", "error": "none", "loglevel": "warning"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := resolveXrayLogPaths(json_util.RawMessage(tt.in))
			var got map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("unmarshal %s: %v", out, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for key, want := range tt.want {
				if got[key] != want {
					t.Fatalf("key %q: got %v, want %v", key, got[key], want)
				}
			}
			for key := range got {
				if strings.EqualFold(key, "access") && key != "access" {
					t.Fatalf("case variant %q survived in %v", key, got)
				}
				if strings.EqualFold(key, "error") && key != "error" {
					t.Fatalf("case variant %q survived in %v", key, got)
				}
			}
		})
	}
}
