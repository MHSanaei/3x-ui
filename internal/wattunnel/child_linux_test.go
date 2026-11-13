//go:build linux

package wattunnel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

func TestLinuxChildDropsRootAndLimitsCapabilities(t *testing.T) {
	binary := os.Getenv("WATTUN_E2E_BINARY")
	if binary == "" || os.Geteuid() != 0 {
		t.Skip("requires root and a native Linux engine")
	}
	for _, carrier := range []string{"tcp", "icmp"} {
		t.Run(carrier, func(t *testing.T) {
			mgr := testManager(t, binary)
			t.Cleanup(mgr.StopAll)
			cfg := Config{Role: "server", Carrier: carrier, Endpoint: "127.0.0.1", Listen: "127.0.0.1", Port: 81, Ports: []Port{{443, 443, "tcp"}}}
			spec := Spec{GUID: uuid.NewString(), Owner: "test", Revision: 1, Enabled: true, Config: cfg, Secret: "credential-test-key"}
			if err := mgr.Apply(context.Background(), spec); err != nil {
				t.Fatal(err)
			}
			mgr.mu.Lock()
			pid := mgr.instances[spec.GUID].cmd.Process.Pid
			mgr.mu.Unlock()
			raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
			if err != nil {
				t.Fatal(err)
			}
			expected := uint64(1) << unix.CAP_NET_BIND_SERVICE
			if carrier == "icmp" {
				expected = uint64(1) << unix.CAP_NET_RAW
			}
			checkedUID, checkedCaps := false, false
			for _, line := range strings.Split(string(raw), "\n") {
				fields := strings.Fields(line)
				if len(fields) == 0 {
					continue
				}
				switch fields[0] {
				case "Uid:":
					if len(fields) != 5 || fields[1] == "0" || fields[2] == "0" || fields[3] == "0" || fields[4] == "0" {
						t.Fatalf("child retained root: %s", line)
					}
					checkedUID = true
				case "CapEff:":
					value, err := strconv.ParseUint(fields[1], 16, 64)
					if err != nil || value != expected {
						t.Fatalf("child capabilities %s, expected %x", fields[1], expected)
					}
					checkedCaps = true
				}
			}
			if !checkedUID || !checkedCaps {
				t.Fatal("could not inspect child credentials")
			}
		})
	}
}

func TestLinuxParentDeathTerminatesChild(t *testing.T) {
	if os.Getenv("WATTUN_E2E_BINARY") == "" || os.Geteuid() != 0 {
		t.Skip("requires root Linux process fixture")
	}
	if dir := os.Getenv("WATTUN_PARENT_DEATH_HELPER"); dir != "" {
		cmd := exec.Command("/bin/sleep", "60")
		if err := prepareChild(cmd, Config{Carrier: "tcp"}, "tcp", dir); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		fmt.Println(cmd.Process.Pid)
		os.Exit(0)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helper := exec.Command(executable, "-test.run=^TestLinuxParentDeathTerminatesChild$")
	helper.Env = append(os.Environ(), "WATTUN_PARENT_DEATH_HELPER="+t.TempDir())
	out, err := helper.Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(raw))
		if len(fields) > 2 && fields[2] == "Z" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = unix.Kill(pid, unix.SIGKILL)
	t.Fatal("child survived abrupt supervisor exit")
}
