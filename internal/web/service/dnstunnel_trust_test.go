package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The resolver test runs during a shutdown on the users' ISP, often over plain HTTP, so
// its command must run only the script the panel made.
func TestDnsTunnelCommandRefusesASwappedScript(t *testing.T) {
	py := pythonForTest(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	fetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches++
		_, _ = w.Write([]byte("open(r\"" + marker + "\", 'w').write('x')\n"))
	}))
	defer srv.Close()
	var s DnsTunnelService
	cmd, err := s.NewCommand(DnsTunnelCommandRequest{Zone: "t.example.com"}, srv.URL+"/p/")
	if err != nil {
		t.Fatal(err)
	}

	code := strings.TrimSuffix(strings.TrimPrefix(cmd.Unix, `python3 -c "`), `"`)
	run := exec.Command(py, "-c", code)
	run.Env = append(os.Environ(), "NO_PROXY=*", "no_proxy=*")
	out, _ := run.CombinedOutput()

	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("the command ran a script the panel did not make; output %q", out)
	}
	if !strings.Contains(string(out), "make a new one on the DNS tunnel page") || fetches != 1 {
		t.Errorf("output %q after %d fetches, want one fetch and the mismatch notice", out, fetches)
	}
}
