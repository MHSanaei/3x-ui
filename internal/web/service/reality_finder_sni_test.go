package service

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const realityFinderSniDriver = `
import json, sys
script, mode, port = sys.argv[1], sys.argv[2], int(sys.argv[3])
g = {"__name__": "finder"}
exec(open(script, encoding="utf-8").read(), g)
if mode == "through":
    print(json.dumps(g["sni_through"]("127.0.0.1", port, "name.example", timeout=1, tries=1)))
else:
    ctx = {"server_ip": "127.0.0.1", "server_port": port, "known_sni": "known.example", "timeout": 1}
    facts = {"ms": 1, "issuer": "", "expires": "", "names": []}
    rows = [g["finish"]("198.51.100.1", f"name{i}.example", facts, ctx, dns=["198.51.100.1"]) for i in range(4)]
    router = g["sni_router_check"](rows, ctx)
    print(json.dumps({"router": router, "filtered": [r["filtered"] for r in rows]}))
`

// runRealityFinderSniDriver runs the script the panel really serves against a local port.
func runRealityFinderSniDriver(t *testing.T, mode, port string) string {
	t.Helper()
	python := findWorkingPython(t)
	script := realityFinderBuildScript(RealityFinderCommandRequest{Server: "203.0.113.10", Port: 443}, "http://127.0.0.1/report")
	path := filepath.Join(t.TempDir(), "finder.py")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(python, "-c", realityFinderSniDriver, path, mode, port).CombinedOutput()
	if err != nil {
		t.Fatalf("driver failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// serveRealityFinderSni answers every connection on a local port with handle, and gives that port.
func serveRealityFinderSni(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handle(conn)
		}
	}()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

// readClientHello consumes the whole first record, so closing the socket cannot turn into a reset.
func readClientHello(conn net.Conn) bool {
	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		return false
	}
	_, err := io.CopyN(io.Discard, conn, int64(header[3])<<8|int64(header[4]))
	return err == nil
}

func resetRealityFinderConn(conn net.Conn) {
	_ = conn.(*net.TCPConn).SetLinger(0)
	_ = conn.Close()
}

func realityFinderTestCert(t *testing.T) tls.Certificate {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv.TLS.Certificates[0]
}

// A name is filtered only when the ISP cuts it. A dest that refuses a name with a TLS
// alert proves the name crossed the network, so it must not read as filtered.
func TestRealityFinderSniThroughTellsARefusalFromACut(t *testing.T) {
	cert := realityFinderTestCert(t)
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	afterHello := func(answer func(net.Conn)) func(net.Conn) {
		return func(conn net.Conn) {
			defer conn.Close()
			if readClientHello(conn) {
				answer(conn)
			}
		}
	}

	cases := []struct {
		name    string
		handle  func(net.Conn)
		through string
	}{
		{"handshake completes", func(conn net.Conn) {
			defer conn.Close()
			_ = tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}}).Handshake()
		}, "true"},
		{"unrecognized_name alert", afterHello(func(conn net.Conn) {
			_, _ = conn.Write([]byte{21, 3, 3, 0, 2, 2, 112})
		}), "true"},
		{"handshake_failure alert", afterHello(func(conn net.Conn) {
			_, _ = conn.Write([]byte{21, 3, 3, 0, 2, 2, 40})
		}), "true"},
		{"reset after the name", func(conn net.Conn) {
			if readClientHello(conn) {
				resetRealityFinderConn(conn)
			}
		}, "false"},
		{"silence after the name", afterHello(func(net.Conn) { <-stop }), "false"},
		{"closed after the name", afterHello(func(net.Conn) {}), "false"},
		{"a non-TLS answer", afterHello(func(conn net.Conn) {
			_, _ = conn.Write([]byte("HTTP/1.1 403 Forbidden\r\n\r\n"))
		}), "false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			port := serveRealityFinderSni(t, tc.handle)
			if got := runRealityFinderSniDriver(t, "through", port); got != tc.through {
				t.Errorf("sni_through = %s, want %s", got, tc.through)
			}
		})
	}
}

// The router check drops every filter result, so a dest refusing unknown names must not
// trip it, while a router that really cuts them still must.
func TestRealityFinderRouterCheckIgnoresADestRefusingNames(t *testing.T) {
	cert := realityFinderTestCert(t)
	onlyKnownName := func(refuse func(net.Conn)) func(net.Conn) {
		return func(conn net.Conn) {
			defer conn.Close()
			_ = tls.Server(conn, &tls.Config{GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				if hello.ServerName == "known.example" {
					return &cert, nil
				}
				refuse(conn)
				return nil, errors.New("unknown name")
			}}).Handshake()
		}
	}

	cases := []struct {
		name   string
		refuse func(net.Conn)
		router bool
		cut    any
	}{
		{"dest answers unknown names with an alert", func(net.Conn) {}, false, false},
		{"router resets unknown names", resetRealityFinderConn, true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			port := serveRealityFinderSni(t, onlyKnownName(tc.refuse))
			var got struct {
				Router   bool  `json:"router"`
				Filtered []any `json:"filtered"`
			}
			out := runRealityFinderSniDriver(t, "router", port)
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("driver output %q: %v", out, err)
			}
			if got.Router != tc.router {
				t.Errorf("router = %v, want %v (output %s)", got.Router, tc.router, out)
			}
			for i, filtered := range got.Filtered {
				if filtered != tc.cut {
					t.Errorf("row %d filtered = %v, want %v", i, filtered, tc.cut)
				}
			}
			if len(got.Filtered) != 4 {
				t.Errorf("got %d rows, want 4 (output %s)", len(got.Filtered), out)
			}
		})
	}
}
