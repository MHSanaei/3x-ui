package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCgroupMemoryLimitAt(t *testing.T) {
	const service = "/system.slice/x-ui.service"
	tests := []struct {
		name       string
		selfCgroup string
		files      map[string]string
		want       int64
		wantOK     bool
	}{
		{
			name:       "systemd MemoryMax inside a container limit",
			selfCgroup: "0::" + service + "\n",
			files: map[string]string{
				"root/memory.max":                "2147483648\n",
				"root/system.slice/memory.max":   "max\n",
				"root" + service + "/memory.max": "681574400\n",
			},
			want: 681574400, wantOK: true,
		},
		{
			name:       "systemd MemoryMax on a bare host, root has no memory.max",
			selfCgroup: "0::" + service + "\n",
			files: map[string]string{
				"root/system.slice/memory.max":   "max\n",
				"root" + service + "/memory.max": "681574400\n",
			},
			want: 681574400, wantOK: true,
		},
		{
			name:       "container limit tighter than the service limit",
			selfCgroup: "0::" + service + "\n",
			files: map[string]string{
				"root/memory.max":                "536870912\n",
				"root" + service + "/memory.max": "681574400\n",
			},
			want: 536870912, wantOK: true,
		},
		{
			name:       "no limit at any level",
			selfCgroup: "0::" + service + "\n",
			files: map[string]string{
				"root/memory.max":                "max\n",
				"root" + service + "/memory.max": "max\n",
			},
		},
		{
			name:       "cgroup v1 limit on the service, unlimited sentinel at the root",
			selfCgroup: "12:cpu,cpuacct:" + service + "\n7:memory:" + service + "\n",
			files: map[string]string{
				"root/memory/memory.limit_in_bytes":                "9223372036854771712\n",
				"root/memory" + service + "/memory.limit_in_bytes": "681574400\n",
			},
			want: 681574400, wantOK: true,
		},
		{
			name:       "cgroup outside the namespace never reads above the root",
			selfCgroup: "0::/../../elsewhere\n",
			files: map[string]string{
				"memory.max":      "1048576\n",
				"root/memory.max": "2147483648\n",
			},
			want: 2147483648, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tt.files {
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, ok := cgroupMemoryLimitAt(tt.selfCgroup, filepath.Join(dir, "root"))
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("cgroupMemoryLimitAt() = %d, %v; want %d, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
