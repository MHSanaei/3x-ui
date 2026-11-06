//go:build windows

package childproc

import (
	"os"
	"os/exec"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsJobAndConfirmedStop(t *testing.T) {
	if os.Getenv("XUI_CHILD_JOB_HELPER") == "1" {
		time.Sleep(time.Minute)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestWindowsJobAndConfirmedStop$")
	cmd.Env = append(os.Environ(), "XUI_CHILD_JOB_HELPER=1")
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { _ = Stop(cmd, done, time.Second, time.Second) })
	if err = Attach(cmd); err != nil {
		t.Fatal(err)
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	var inJob int32
	result, _, callErr := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob").Call(uintptr(h), uintptr(killOnExitJob), uintptr(unsafe.Pointer(&inJob)))
	if result == 0 || inJob == 0 {
		err = callErr
		t.Fatalf("not assigned to kill-on-exit job: %v", err)
	}
	if err = Stop(cmd, done, time.Second, time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	default:
		t.Fatal("Stop returned without confirmed child exit")
	}
}
