// Package nodee2e drives a real master panel and a real node panel, each its own
// process, through the node-sync paths. Gated by XUI_NODE_E2E_BINARY.
package nodee2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

const settleTimeout = 30 * time.Second

func panelBinary(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("XUI_NODE_E2E_BINARY")
	if bin == "" {
		t.Skip("XUI_NODE_E2E_BINARY not set; run `make node-e2e`")
	}
	abs, err := filepath.Abs(bin)
	if err != nil {
		t.Fatalf("resolve %s: %v", bin, err)
	}
	return abs
}

type panel struct {
	t      *testing.T
	name   string
	bin    string
	dir    string
	port   int
	token  string
	cmd    *exec.Cmd
	logOut *os.File
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func (p *panel) env() []string {
	return append(os.Environ(),
		"XUI_DB_FOLDER="+filepath.Join(p.dir, "db"),
		"XUI_LOG_FOLDER="+filepath.Join(p.dir, "log"),
		"XUI_BIN_FOLDER="+filepath.Join(p.dir, "bin"),
		"XUI_ENABLE_FAIL2BAN=false",
		"MSYS_NO_PATHCONV=1",
	)
}

func (p *panel) cli(args ...string) string {
	p.t.Helper()
	cmd := exec.Command(p.bin, args...)
	cmd.Env = p.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		p.t.Fatalf("%s %v: %v\n%s", p.name, args, err, out)
	}
	return string(out)
}

var apiTokenLine = regexp.MustCompile(`(?m)^apiToken:\s*(\S+)`)

func (p *panel) mintToken(name, scope string) string {
	p.t.Helper()
	out := p.cli("setting", "-getApiToken", "-tokenName", name, "-tokenScope", scope)
	m := apiTokenLine.FindStringSubmatch(out)
	if m == nil {
		p.t.Fatalf("%s: no apiToken in output:\n%s", p.name, out)
	}
	return m[1]
}

// newPanel prepares a panel's database: credentials, a private port, its own
// sub-server port (two panels on one host would race for 2096) and an admin token.
func newPanel(t *testing.T, bin, name string) *panel {
	t.Helper()
	p := preparePanel(t, bin, name)
	p.token = p.mintToken("e2e-driver", "admin")
	return p
}

// sharedDBMu guards the process-global database handle the harness borrows
// while the scopes run in parallel.
var sharedDBMu sync.Mutex

func preparePanel(t *testing.T, bin, name string) *panel {
	t.Helper()
	p := &panel{t: t, name: name, bin: bin, dir: t.TempDir(), port: freePort(t)}
	for _, d := range []string{"db", "log", "bin"} {
		if err := os.MkdirAll(filepath.Join(p.dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p.cli("setting", "-username", "e2e", "-password", "e2e-pass", "-port", strconv.Itoa(p.port), "-webBasePath", "/")
	sharedDBMu.Lock()
	defer sharedDBMu.Unlock()
	if err := database.InitDB(filepath.Join(p.dir, "db", "x-ui.db")); err != nil {
		t.Fatalf("%s: open db: %v", name, err)
	}
	db := database.GetDB()
	db.Exec("DELETE FROM settings WHERE key = ?", "subPort")
	if err := db.Exec("INSERT INTO settings(key, value) VALUES (?, ?)", "subPort", strconv.Itoa(freePort(t))).Error; err != nil {
		t.Fatalf("%s: set subPort: %v", name, err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatalf("%s: close db: %v", name, err)
	}
	t.Cleanup(p.stop)
	return p
}

func (p *panel) start() {
	p.t.Helper()
	logOut, err := os.OpenFile(filepath.Join(p.dir, "stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		p.t.Fatal(err)
	}
	p.logOut = logOut
	p.cmd = exec.Command(p.bin, "run")
	p.cmd.Env = p.env()
	p.cmd.Stdout = logOut
	p.cmd.Stderr = logOut
	if err := p.cmd.Start(); err != nil {
		p.t.Fatalf("%s: start: %v", p.name, err)
	}
	eventually(p.t, settleTimeout, p.name+" answers /server/status", func() (bool, string) {
		env, err := p.try(http.MethodGet, "/panel/api/server/status", nil)
		if err != nil {
			return false, err.Error()
		}
		return env.Success, env.Msg
	})
}

func (p *panel) stop() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Kill()
	_, _ = p.cmd.Process.Wait()
	p.cmd = nil
	if p.logOut != nil {
		_ = p.logOut.Close()
		p.logOut = nil
	}
	if p.t.Failed() {
		if b, err := os.ReadFile(filepath.Join(p.dir, "stdout.log")); err == nil {
			tail := string(b)
			if len(tail) > 6000 {
				tail = tail[len(tail)-6000:]
			}
			p.t.Logf("---- %s stdout tail ----\n%s", p.name, tail)
		}
	}
}

// deleteInboundRow simulates a node that lost an inbound (restore, reinstall)
// while stopped; it must not run against a live panel.
func (p *panel) deleteInboundRow(id int) {
	p.t.Helper()
	if p.cmd != nil {
		p.t.Fatalf("%s: deleteInboundRow on a running panel", p.name)
	}
	sharedDBMu.Lock()
	defer sharedDBMu.Unlock()
	if err := database.InitDB(filepath.Join(p.dir, "db", "x-ui.db")); err != nil {
		p.t.Fatalf("%s: open db: %v", p.name, err)
	}
	defer func() { _ = database.CloseDB() }()
	db := database.GetDB()
	for _, q := range []string{"DELETE FROM client_inbounds WHERE inbound_id = ?", "DELETE FROM client_traffics WHERE inbound_id = ?", "DELETE FROM inbounds WHERE id = ?"} {
		if err := db.Exec(q, id).Error; err != nil {
			p.t.Fatalf("%s: %s: %v", p.name, q, err)
		}
	}
}

func (p *panel) url() string { return "http://127.0.0.1:" + strconv.Itoa(p.port) }

type envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

func (p *panel) try(method, path string, body any) (*envelope, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, p.url()+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, raw)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode %s: %w (%s)", path, err, raw)
	}
	return &env, nil
}

// call fails the test on transport errors or success:false.
func (p *panel) call(method, path string, body any) json.RawMessage {
	p.t.Helper()
	env, err := p.try(method, path, body)
	if err != nil {
		p.t.Fatalf("%s %s %s: %v", p.name, method, path, err)
	}
	if !env.Success {
		p.t.Fatalf("%s %s %s: success=false msg=%q", p.name, method, path, env.Msg)
	}
	return env.Obj
}

func eventually(t *testing.T, timeout time.Duration, what string, check func() (bool, string)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := ""
	for {
		ok, detail := check()
		if ok {
			return
		}
		last = detail
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s; last: %s", timeout, what, last)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// inboundView is the subset of an inbound row the scenarios assert on.
type inboundView struct {
	Id       int             `json:"id"`
	Remark   string          `json:"remark"`
	Enable   bool            `json:"enable"`
	Port     int             `json:"port"`
	Tag      string          `json:"tag"`
	NodeID   *int            `json:"nodeId"`
	Settings json.RawMessage `json:"settings"`
}

type clientEntry map[string]any

func (c clientEntry) email() string { s, _ := c["email"].(string); return s }

func (ib inboundView) clients() []clientEntry {
	raw := ib.Settings
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		raw = json.RawMessage(asString)
	}
	var s struct {
		Clients []clientEntry `json:"clients"`
	}
	_ = json.Unmarshal(raw, &s)
	return s.Clients
}

func (ib inboundView) emails() []string {
	out := []string{}
	for _, c := range ib.clients() {
		out = append(out, c.email())
	}
	return out
}

func (ib inboundView) client(email string) clientEntry {
	for _, c := range ib.clients() {
		if strings.EqualFold(c.email(), email) {
			return c
		}
	}
	return nil
}

func (p *panel) inbounds() []inboundView {
	p.t.Helper()
	var list []inboundView
	if err := json.Unmarshal(p.call(http.MethodGet, "/panel/api/inbounds/list", nil), &list); err != nil {
		p.t.Fatalf("%s: decode inbound list: %v", p.name, err)
	}
	return list
}

func (p *panel) inboundOnPort(port int) (inboundView, bool) {
	p.t.Helper()
	for _, ib := range p.inbounds() {
		if ib.Port == port {
			return ib, true
		}
	}
	return inboundView{}, false
}

const tcpStream = `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`

func vlessInbound(remark string, port int, nodeID *int, clients ...map[string]any) map[string]any {
	if clients == nil {
		clients = []map[string]any{}
	}
	settings, _ := json.Marshal(map[string]any{"clients": clients, "decryption": "none"})
	body := map[string]any{
		"remark": remark, "enable": true, "port": port, "protocol": "vless",
		"settings": string(settings), "streamSettings": tcpStream, "sniffing": `{}`,
	}
	if nodeID != nil {
		body["nodeId"] = *nodeID
	}
	return body
}

func vlessClient(email string) map[string]any {
	return map[string]any{"email": email, "enable": true, "id": newUUID(email)}
}

// newUUID derives a stable, valid UUID from a label so failures are reproducible.
func newUUID(label string) string {
	var b [16]byte
	copy(b[:], []byte(label+"________________"))
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
