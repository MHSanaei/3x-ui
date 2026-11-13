package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The command runs on the admin's device over the users' (hostile) ISP, often over plain
// HTTP, so it must run only the exact script the panel made for it.
func TestRealityFinderCommandRefusesASwappedScript(t *testing.T) {
	python := findWorkingPython(t)
	resetRealityFinder(t, time.Now())
	marker := filepath.Join(t.TempDir(), "pwned")
	swapped := "open(" + `r"` + marker + `"` + ", 'w').write('x')\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(swapped))
	}))
	defer srv.Close()
	cmd, err := (&RealityFinderService{}).NewCommand(
		RealityFinderCommandRequest{Server: "203.0.113.10", Port: 443}, srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}

	code := strings.TrimSuffix(strings.TrimPrefix(cmd.Unix, `python3 -c "`), `"`)
	out, _ := exec.Command(python, "-c", code).CombinedOutput()

	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("the command ran a script the panel did not make; output %q", out)
	}
	if !strings.Contains(string(out), "did not match") {
		t.Errorf("output = %q, want the mismatch notice", out)
	}
	script, err := (&RealityFinderService{}).Script(cmd.Token)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(script))
	if !strings.Contains(cmd.Unix, "'"+hex.EncodeToString(sum[:])+"'") {
		t.Errorf("command %q does not accept the script the panel serves", cmd.Unix)
	}
}

func TestRealityFinderScansTheNodeAnInboundIsDeployedTo(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	resetRealityFinder(t, time.Now())
	db := database.GetDB()
	node := &model.Node{Name: "de-1", Address: "node1.example.com", Port: 2053, ApiToken: "x"}
	private := &model.Node{Name: "lan", Address: "10.0.0.7", Port: 2053, ApiToken: "x"}
	for _, n := range []*model.Node{node, private} {
		if err := db.Create(n).Error; err != nil {
			t.Fatal(err)
		}
	}

	cmd, err := (&RealityFinderService{}).NewCommand(
		RealityFinderCommandRequest{Port: 443, NodeId: &node.Id}, "https://panel.example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Server != "node1.example.com" {
		t.Errorf("server = %q, want the node's address", cmd.Server)
	}

	_, err = (&RealityFinderService{}).NewCommand(
		RealityFinderCommandRequest{Port: 443, NodeId: &private.Id}, "https://panel.example.com/")
	if !errors.Is(err, errRealityFinderNoServer) {
		t.Errorf("node on a private address: err = %v, want errRealityFinderNoServer", err)
	}
}

// findWorkingPython skips past the Windows Store stub, which is on PATH but runs nothing.
func findWorkingPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if out, err := exec.Command(name, "-c", "print(7*6)").Output(); err == nil && strings.TrimSpace(string(out)) == "42" {
			return name
		}
	}
	t.Skip("no working python on PATH")
	return ""
}
