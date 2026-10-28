package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Extract definitions only: sourcing x-ui.sh would run its privileged entry point.
func menuBasePathFunction(source, name string) string {
	_, body, ok := strings.Cut("\n"+source, "\n"+name+"() {\n")
	if !ok {
		return ""
	}
	body, _, ok = strings.Cut(body, "\n}\n")
	if !ok {
		return ""
	}
	return name + "() {\n" + body + "\n}\n"
}

const menuBasePathCLI = `#!/usr/bin/env bash
set -eu
state=$TEST_MENU_DIR
[[ "${1:-}" == setting ]] || exit 90
case "${2:-}" in
    -show)
        echo read >> "$state/events"
        mode=$TEST_MENU_READ_MODE
        if [[ "$mode" == post_* ]]; then
            if [[ -s "$state/writes" ]]; then mode=${mode#post_}; else mode=normal; fi
        fi
        case "$mode" in
            failure) echo 'controlled read failure' >&2; exit 1 ;;
            empty) exit 0 ;;
            diagnostic) echo 'get webBasePath failed, error info: controlled failure' ;;
        esac
        echo 'current panel settings as follows:'
        echo 'hasDefaultCredential: false'
        echo 'port: 54321'
        case "$mode" in
            missing) ;;
            duplicate) printf 'webBasePath: /old/\nwebBasePath: /other/\n' ;;
            relative) echo 'webBasePath: relative' ;;
            *) printf 'webBasePath: %s\n' "$(cat "$state/path")" ;;
        esac
        ;;
    -webBasePath)
        [[ $# == 3 ]] || { echo 'Unrelated setting modified' >&2; exit 91; }
        printf '%s\n' "$3" >> "$state/writes"
        echo write >> "$state/events"
        case "$TEST_MENU_UPDATE_MODE" in
            failure) echo 'controlled update failure' >&2; exit 1 ;;
            zero) echo 'Failed to set base URI path: controlled failure'; exit 0 ;;
            no_persistence) echo 'Base URI path set successfully'; exit 0 ;;
        esac
        path=$3
        [[ "$path" == /* ]] || path="/$path"
        [[ "$path" == */ ]] || path="$path/"
        printf '%s' "$path" > "$state/path"
        if [[ "$TEST_MENU_UPDATE_MODE" == write_error ]]; then
            echo 'Failed to set base URI path: controlled failure'
        else
            echo 'Base URI path set successfully'
        fi
        ;;
    *) echo 'Unexpected CLI arguments' >&2; exit 92 ;;
esac
`

func TestMenuWebBasePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("menu regressions require a POSIX environment; run under Linux or WSL")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("bash is required for the menu regression tests")
	}
	sourceBytes, err := os.ReadFile("x-ui.sh")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	driver := `set -u
red= green= yellow= blue= plain=
xui_folder=$TEST_MENU_CLI_DIR
restart() { echo restart >> "$TEST_MENU_DIR/events"; }
gen_random_string() {
    [[ "$1" == 18 ]] || return 1
    echo random >> "$TEST_MENU_DIR/random"
    [[ "$TEST_MENU_RANDOM_MODE" != failure ]] || return 1
    printf '%s\n' "$TEST_MENU_RANDOM_VALUE"
}
`
	for _, name := range []string{"normalize_menu_webbasepath", "confirm_menu_webbasepath", "read_menu_webbasepath", "reset_webbasepath"} {
		definition := menuBasePathFunction(source, name)
		if definition == "" && name == "reset_webbasepath" {
			t.Fatal("menu option 8 handler is missing")
		}
		driver += definition
	}
	driver += "reset_webbasepath\n"

	type menuCase struct {
		name, input, want, message                string
		current, readMode, updateMode, randomMode string
		randomValue                               string
		writes, restarts, randomCalls             int
		failure, spacedCLI                        bool
	}
	cases := []menuCase{
		{name: "random", input: "y\n1\ny\n", want: "/AbCdEf0123456789Gh/", writes: 1, restarts: 1, randomCalls: 1},
		{name: "cancel mode", input: "y\n0\n", message: "Operation canceled"},
		{name: "empty confirmation", input: "\n", message: "Operation canceled"},
		{name: "negative confirmation", input: "NO\n", message: "Operation canceled"},
		{name: "invalid confirmation retries", input: "maybe\nYES\n2\nnew\ny\n", want: "/new/", writes: 1, restarts: 1, message: "Please answer yes or no"},
		{name: "invalid selection retries", input: "y\n3\ninvalid\n2\nnew\ny\n", want: "/new/", writes: 1, restarts: 1, message: "Invalid selection"},
		{name: "cancel custom confirmation", input: "y\n2\nnew\nn\n", message: "Operation canceled"},
		{name: "empty custom confirmation", input: "y\n2\nnew\n\n", message: "Operation canceled"},
		{name: "cancel random confirmation", input: "y\n1\nn\n", randomCalls: 1, message: "Operation canceled"},
		{name: "same normalized path", input: "y\n2\nold\n", message: "No change is necessary"},
		{name: "same short normalized path", current: "/a/", input: "y\n2\na\n", message: "No change is necessary"},
		{name: "existing root", current: "/", input: "y\n2\na\ny\n", want: "/a/", writes: 1, restarts: 1},
		{name: "quoted CLI path", input: "y\n2\nnew\ny\n", want: "/new/", writes: 1, restarts: 1, spacedCLI: true},
		{name: "CLI failure", input: "y\n2\nnew\ny\n", writes: 1, updateMode: "failure", failure: true, message: "Failed to update"},
		{name: "CLI zero-exit failure", input: "y\n2\nnew\ny\n", writes: 1, updateMode: "zero", failure: true, message: "Failed to update"},
		{name: "CLI reports failure after write", input: "y\n2\nnew\ny\n", want: "/new/", writes: 1, updateMode: "write_error", failure: true, message: "Failed to update"},
		{name: "persistence mismatch", input: "y\n2\nnew\ny\n", writes: 1, updateMode: "no_persistence", failure: true, message: "Could not verify"},
		{name: "post-write read failure", input: "y\n2\nnew\ny\n", want: "/new/", writes: 1, readMode: "post_failure", failure: true, message: "Could not verify"},
		{name: "post-write empty read", input: "y\n2\nnew\ny\n", want: "/new/", writes: 1, readMode: "post_empty", failure: true, message: "Could not verify"},
		{name: "post-write getter error", input: "y\n2\nnew\ny\n", want: "/new/", writes: 1, readMode: "post_diagnostic", failure: true, message: "Could not verify"},
		{name: "random generator failure", input: "y\n1\ny\n", randomCalls: 1, randomMode: "failure", failure: true},
		{name: "short random output", input: "y\n1\ny\n", randomCalls: 1, randomValue: "short", failure: true},
		{name: "EOF confirmation", message: "Operation canceled"},
		{name: "EOF selection", input: "y\n", message: "Operation canceled"},
		{name: "EOF custom input", input: "y\n2\n", message: "Operation canceled"},
		{name: "EOF apply confirmation", input: "y\n2\nnew\n", message: "Operation canceled"},
	}
	for _, answer := range []string{"y", "Y", "yes", "Yes", "YES", "  yEs  "} {
		cases = append(cases, menuCase{name: "initial " + answer, input: answer + "\n2\nnew\ny\n", want: "/new/", writes: 1, restarts: 1})
		cases = append(cases, menuCase{name: "apply " + answer, input: "y\n2\nnew\n" + answer + "\n", want: "/new/", writes: 1, restarts: 1})
	}
	for _, path := range []string{"mypanel", "/mypanel", "/mypanel/", "admin-2026", "a", "my_panel", "private/admin", "Private/ADMIN-2026", "-private/_admin"} {
		want := "/" + strings.Trim(path, "/") + "/"
		cases = append(cases, menuCase{name: "custom " + path, input: "y\n2\n" + path + "\ny\n", want: want, writes: 1, restarts: 1})
	}
	for _, path := range []string{"", "/", ".", "..", "a/./b", "a/../b", "a//b", "//admin", "admin//", " admin", "admin ", "a b", "a\tb", "a\rb", "a\x01b", "a?b", "a#b", "a\\b", "%2e%2e", "a;b", "a'b", "a\"b", "$(id)", "`id`", "a:b", "a+b"} {
		cases = append(cases, menuCase{name: "invalid " + path, input: "y\n2\n" + path + "\nvalid\ny\n", want: "/valid/", writes: 1, restarts: 1, message: "Invalid Web Base Path"})
	}
	for _, mode := range []string{"failure", "empty", "missing", "duplicate", "relative", "diagnostic"} {
		cases = append(cases, menuCase{name: "initial read " + mode, input: "y\n2\nnew\ny\n", readMode: mode, failure: true, message: "Failed to read"})
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cliDir := dir
			if tt.spacedCLI {
				cliDir = filepath.Join(dir, "panel cli")
				if err := os.Mkdir(cliDir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			current := tt.current
			if current == "" {
				current = "/old/"
			}
			unchanged := map[string]string{"port": "54321", "username": "existing-user", "password": "existing-password", "cert": "/cert/panel.pem", "key": "/cert/panel.key", "db": "existing-db"}
			for name, value := range unchanged {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for name, value := range map[string]string{"path": current, "writes": "", "events": "", "random": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(cliDir, "x-ui"), []byte(menuBasePathCLI), 0o700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bash, "-c", driver)
			cmd.Stdin = strings.NewReader(tt.input)
			cmd.Env = append(os.Environ(), "TEST_MENU_DIR="+dir, "TEST_MENU_CLI_DIR="+cliDir, "TEST_MENU_READ_MODE="+tt.readMode, "TEST_MENU_UPDATE_MODE="+tt.updateMode, "TEST_MENU_RANDOM_MODE="+tt.randomMode)
			randomValue := tt.randomValue
			if randomValue == "" {
				randomValue = "AbCdEf0123456789Gh"
			}
			cmd.Env = append(cmd.Env, "TEST_MENU_RANDOM_VALUE="+randomValue)
			output, runErr := cmd.CombinedOutput()
			if (runErr != nil) != tt.failure {
				t.Errorf("execution error = %v, want failure %v; output:\n%s", runErr, tt.failure, output)
			}
			read := func(name string) string {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			want := tt.want
			if want == "" {
				want = current
			}
			if got := read("path"); got != want {
				t.Errorf("stored path = %q, want %q; output:\n%s", got, want, output)
			}
			if got := strings.Count(read("writes"), "\n"); got != tt.writes {
				t.Errorf("setting writes = %d, want %d", got, tt.writes)
			}
			if got := strings.Count(read("events"), "restart\n"); got != tt.restarts {
				t.Errorf("restarts = %d, want %d", got, tt.restarts)
			}
			if tt.restarts == 1 && read("events") != "read\nwrite\nread\nrestart\n" {
				t.Errorf("unexpected apply/verify/restart order: %q", read("events"))
			}
			if got := strings.Count(read("random"), "\n"); got != tt.randomCalls {
				t.Errorf("random generations = %d, want %d", got, tt.randomCalls)
			}
			if tt.writes == 1 && read("writes") != want+"\n" && !tt.failure {
				t.Errorf("CLI path argument = %q, want %q", read("writes"), want)
			}
			if tt.message != "" && !strings.Contains(string(output), tt.message) {
				t.Errorf("missing notice %q in:\n%s", tt.message, output)
			}
			if (tt.failure || tt.restarts == 0) && strings.Contains(string(output), "updated successfully") {
				t.Errorf("false success notice:\n%s", output)
			}
			for name, value := range unchanged {
				if got := read(name); got != value {
					t.Errorf("unrelated %s changed to %q", name, got)
				}
			}
		})
	}
}
