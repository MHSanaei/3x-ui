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

// The command runs on the admin's device over the filtered ISP, often over plain HTTP, so
// both the direct and the pinned (?via=) command must run only the script the panel made.
func TestCleanIpCommandRefusesASwappedScript(t *testing.T) {
	py := pythonForTest(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	var fetched []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched = append(fetched, r.URL.RequestURI())
		_, _ = w.Write([]byte("open(r\"" + marker + "\", 'w').write('x')\n"))
	}))
	defer srv.Close()
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	var s CleanIpService
	req := CleanIpCommandRequest{Host: "cdn.example.com", Ips: []string{"127.0.0.1"}}
	direct, err := s.NewCommand(req, "http://127.0.0.1:"+port+"/secret/")
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := s.NewCommand(req, "http://panel.invalid:"+port+"/secret/")
	if err != nil {
		t.Fatal(err)
	}

	for name, line := range map[string]string{"direct": direct.Unix, "via": cmd.UnixVia} {
		fetched = nil
		code := strings.TrimSuffix(strings.TrimPrefix(line, `python3 -c "`), `"`)
		run := exec.Command(py, "-c", code)
		run.Env = append(os.Environ(), "NO_PROXY=*", "no_proxy=*")
		out, _ := run.CombinedOutput()

		if _, err := os.Stat(marker); err == nil {
			t.Fatalf("%s: the command ran a script the panel did not make; output %q", name, out)
		}
		if !strings.Contains(string(out), "did not match") || len(fetched) != 1 {
			t.Errorf("%s: output %q after fetching %v, want one fetch and the mismatch notice", name, out, fetched)
		}
	}
	if want := "/secret/cleanip/run/" + cmd.Token + "/script.py?via=127.0.0.1"; !strings.Contains(cmd.UnixVia, "'http://panel.invalid:"+port+want+"'") {
		t.Errorf("via command %q does not fetch %s from the panel's domain", cmd.UnixVia, want)
	}
}
