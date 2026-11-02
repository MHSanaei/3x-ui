package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func scriptFunction(t *testing.T, script, name string) string {
	t.Helper()
	source, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	_, body, ok := strings.Cut("\n"+string(source), "\n"+name+"() {\n")
	if !ok {
		t.Fatalf("%s: missing function %s", script, name)
	}
	body, _, ok = strings.Cut(body, "\n}\n")
	if !ok {
		t.Fatalf("%s: missing closing brace for %s", script, name)
	}
	return name + "() {\n" + body + "\n}\n"
}

const settingsCLIMock = `#!/bin/bash
set -eu
state=$TEST_SETTINGS_DIR
if [[ "$1" == migrate ]]; then exit 0; fi
shift
case "$1" in
    -show)
        case "$TEST_READ_MODE" in
            failure) exit 1 ;;
            empty) exit 0 ;;
            diagnostic) echo 'get webBasePath failed, error info: database unavailable' ;;
        esac
        echo 'current panel settings as follows:'
        echo "hasDefaultCredential: $(cat "$state/default")"
        echo "port: $(cat "$state/port")"
        case "$TEST_READ_MODE" in
            missing_field) ;;
            malformed) echo 'webBasePath: not-an-absolute-path' ;;
            duplicate) printf 'webBasePath: /\nwebBasePath: /a/\n' ;;
            *)
                path=$(cat "$state/path")
                printf 'webBasePath: %s\n' "${path:-/}" ;;
        esac
        exit 0 ;;
    -getCert) echo 'cert: /tmp/panel.example/fullchain.pem'; exit 0 ;;
    -getApiToken) echo 'apiToken: test-token'; exit 0 ;;
esac
printf 'write\n' >> "$state/writes"
while [[ $# -gt 0 ]]; do
    key=$1 value=$2
    shift 2
    case "$key" in
        -webBasePath)
            printf '%s\n' "$value" >> "$state/path-writes"
            [[ "$value" == /* ]] || value="/$value"
            [[ "$value" == */ ]] || value="$value/"
            printf '%s' "$value" > "$state/path" ;;
        -username) printf '%s' "$value" > "$state/username" ;;
        -password) printf '%s' "$value" > "$state/password" ;;
        -port) printf '%s' "$value" > "$state/port" ;;
        *) echo "unexpected setting: $key" >&2; exit 98 ;;
    esac
done
if [[ "$(cat "$state/username")" != admin && "$(cat "$state/password")" != admin ]]; then
    echo false > "$state/default"
fi
`

func runScriptConfig(t *testing.T, script, path string, defaults bool, mode string, runs int, continueSetup bool) (string, string, error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("installer regressions require a POSIX environment; run them under WSL")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is required for the isolated installer regression tests")
	}
	dir := t.TempDir()
	username, password := "custom-user", "custom-password"
	if defaults {
		username, password = "admin", "admin"
	}
	for name, value := range map[string]string{
		"path": path, "username": username, "password": password,
		"default": fmt.Sprint(defaults), "port": "54321", "x-ui": settingsCLIMock,
		"writes": "", "path-writes": "", "restarts": "", "lifecycle": "", "x-ui.service": "test unit",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	function := "config_after_update"
	if script == "install.sh" {
		function = "config_after_install"
	}
	driver := `set -u
red= green= yellow= blue= plain=
xui_folder=$TEST_SETTINGS_DIR
xui_service=$TEST_SETTINGS_DIR
tag_version=test
script_ref=main
NONINTERACTIVE=1
release=ubuntu
SSL_SCHEME=https
SSL_HOST=panel.example
curl() { printf '203.0.113.10\n200\n'; }
systemctl() { printf '%s\n' "$*" >> "$TEST_SETTINGS_DIR/restarts"; }
rc-service() { printf '%s\n' "$*" >> "$TEST_SETTINGS_DIR/restarts"; }
prompt_and_setup_ssl() { printf '%s\n' "$*" > "$TEST_SETTINGS_DIR/ssl"; }
write_install_result() { printf '%s\n' "$@" > "$TEST_SETTINGS_DIR/result"; }
install_postgres_local() { echo 'unexpected PostgreSQL installation' >&2; exit 97; }
chown() { :; }
chmod() { :; }
[() { if [[ "${2:-}" == /etc/* ]]; then return 1; fi; builtin [ "$@"; }
_install_xui_service_unit() { printf 'unit\n' >> "$TEST_SETTINGS_DIR/lifecycle"; }
setup_fail2ban() { printf 'fail2ban\n' >> "$TEST_SETTINGS_DIR/lifecycle"; }
`
	driver += scriptFunction(t, script, "gen_random_string") + scriptFunction(t, script, function)
	if continueSetup {
		parent := "update_x-ui"
		if script == "install.sh" {
			parent = "install_x-ui"
		}
		body := scriptFunction(t, script, parent)
		start := strings.Index(body, "\n    "+function)
		if start < 0 {
			t.Fatalf("%s: missing configuration call in %s", script, parent)
		}
		driver += "finish_setup() {" + body[start:] + "finish_setup\n"
	} else {
		driver += fmt.Sprintf("for ((i=0; i<%d; i++)); do %s || exit $?; done\n", runs, function)
	}
	cmd := exec.Command(bash, "-c", driver)
	cmd.Dir = dir
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "XUI_") && !strings.HasPrefix(value, "TEST_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "TEST_SETTINGS_DIR="+dir, "TEST_READ_MODE="+mode,
		"XUI_DB_TYPE=sqlite", "XUI_PANEL_PORT=41234")
	out, err := cmd.CombinedOutput()
	return dir, string(out), err
}

func readScriptState(t *testing.T, dir, name string) string {
	t.Helper()
	value, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(value)
}

func TestScriptConfigPreservesCustomBasePaths(t *testing.T) {
	for _, script := range []string{"install.sh", "update.sh"} {
		for _, path := range []string{"/", "", "/a/", "/ab/", "/abc/", "/mypanel/", "/failed/", "/my panel/"} {
			for _, defaults := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/path=%q/defaults=%t", script, path, defaults), func(t *testing.T) {
					dir, out, err := runScriptConfig(t, script, path, defaults, "", 2, false)
					if err != nil {
						t.Fatalf("configure twice: %v\n%s", err, out)
					}
					actualPath := readScriptState(t, dir, "path")
					initialize := path == "" || path == "/"
					if initialize {
						if !regexp.MustCompile(`^/[A-Za-z0-9]{18}/$`).MatchString(actualPath) {
							t.Errorf("initialized path = %q, want secure 18-character path", actualPath)
						}
						if n := strings.Count(readScriptState(t, dir, "path-writes"), "\n"); n != 1 {
							t.Errorf("path initialized %d times across two runs, want once", n)
						}
					} else if actualPath != path || readScriptState(t, dir, "path-writes") != "" {
						t.Errorf("custom path %q became %q; writes=%q", path, actualPath, readScriptState(t, dir, "path-writes"))
					}
					for _, field := range []string{"username", "password"} {
						actual := readScriptState(t, dir, field)
						if defaults && script == "install.sh" {
							if !regexp.MustCompile(`^[A-Za-z0-9]{10}$`).MatchString(actual) || actual == "admin" {
								t.Errorf("%s was not securely initialized: %q", field, actual)
							}
						} else {
							want := "admin"
							if !defaults {
								want = "custom-" + field
								if field == "username" {
									want = "custom-user"
								}
							}
							if actual != want {
								t.Errorf("%s = %q, want preserved %q", field, actual, want)
							}
						}
					}
					wantRestarts := ""
					if initialize && script == "update.sh" {
						wantRestarts = "restart x-ui\n"
					}
					if actual := readScriptState(t, dir, "restarts"); actual != wantRestarts {
						t.Errorf("path-related restarts = %q, want %q", actual, wantRestarts)
					}
					wantWrites := 0
					if initialize || (defaults && script == "install.sh") {
						wantWrites = 1
					}
					if n := strings.Count(readScriptState(t, dir, "writes"), "\n"); n != wantWrites {
						t.Errorf("configuration writes = %d, want %d across two runs", n, wantWrites)
					}
					wantPort := "54321"
					if initialize && defaults && script == "install.sh" {
						wantPort = "41234"
					}
					if actual := readScriptState(t, dir, "port"); actual != wantPort {
						t.Errorf("port = %q, want %q", actual, wantPort)
					}
					if !initialize {
						if _, err := os.Stat(filepath.Join(dir, "ssl")); !os.IsNotExist(err) {
							t.Errorf("configured TLS should not be reinitialized: %v", err)
						}
					}
				})
			}
		}
	}
}

func TestScriptConfigRejectsUnreadableSettings(t *testing.T) {
	for _, script := range []string{"install.sh", "update.sh"} {
		for _, mode := range []string{"failure", "empty", "missing_field", "malformed", "duplicate", "diagnostic"} {
			t.Run(script+"/"+mode, func(t *testing.T) {
				dir, out, err := runScriptConfig(t, script, "/", true, mode, 1, false)
				if err == nil || !strings.Contains(out, "Unable to read webBasePath") {
					t.Errorf("settings read should stop with an actionable error; got %v\n%s", err, out)
				}
				for name, want := range map[string]string{"path": "/", "username": "admin", "password": "admin", "writes": "", "restarts": ""} {
					if actual := readScriptState(t, dir, name); actual != want {
						t.Errorf("failed settings read changed %s to %q, want %q", name, actual, want)
					}
				}
			})
		}
	}
}

func TestScriptConfigReadFailureKeepsServiceSetup(t *testing.T) {
	for _, script := range []string{"install.sh", "update.sh"} {
		t.Run(script, func(t *testing.T) {
			dir, out, err := runScriptConfig(t, script, "/a/", true, "failure", 1, true)
			if err != nil || !strings.Contains(out, "Unable to read webBasePath") {
				t.Errorf("service setup should continue after the settings warning: %v\n%s", err, out)
			}
			wantLifecycle, wantServices := "fail2ban\n", ""
			if script == "install.sh" {
				wantLifecycle = "unit\nfail2ban\n"
				wantServices = "daemon-reload\nenable x-ui\nstart x-ui\n"
			}
			for name, want := range map[string]string{
				"path": "/a/", "username": "admin", "password": "admin", "writes": "",
				"lifecycle": wantLifecycle, "restarts": wantServices,
			} {
				if actual := readScriptState(t, dir, name); actual != want {
					t.Errorf("%s = %q, want %q", name, actual, want)
				}
			}
		})
	}
}
