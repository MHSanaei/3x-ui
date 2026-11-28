package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeFail2banLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("requires bash")
	}
	for _, tc := range []struct {
		name, initial, command, wantChoice string
		setup, installed, jail, ssh        bool
	}{
		{name: "Enter declines", command: "export NONINTERACTIVE=0; printf '\\n' | setup_install", wantChoice: "false"},
		{name: "Yes enables", command: "export NONINTERACTIVE=0; printf 'y\\n' | setup_install", wantChoice: "true", setup: true, installed: true, jail: true},
		{name: "unattended unset", command: "setup_install", wantChoice: "false"},
		{name: "unattended false", command: "XUI_ENABLE_FAIL2BAN=false setup_install", wantChoice: "false"},
		{name: "unattended true", command: "XUI_ENABLE_FAIL2BAN=true setup_install", wantChoice: "true", setup: true, installed: true, jail: true},
		{name: "retry configured jail without replacing its ban duration", initial: "legacy", command: "rm \"$TEST_DIR/managed-running\"; XUI_ENABLE_FAIL2BAN=true setup_update; test -e \"$TEST_DIR/managed-running\"", wantChoice: "true", installed: true, jail: true},
		{name: "legacy active keeps configuration", initial: "legacy", command: "setup_update", installed: true, jail: true},
		{name: "update without managed jail", initial: "ssh", command: "setup_update", wantChoice: "false", installed: true, ssh: true},
		{name: "remove and reinstall", initial: "legacy ssh", command: "printf '1\\n' | remove_iplimit; XUI_ENABLE_FAIL2BAN=true setup_update; setup_install", wantChoice: "false", installed: true, ssh: true},
		{name: "remove package and update", initial: "legacy", command: "printf '2\\n' | remove_iplimit; setup_update", wantChoice: "false"},
		{name: "package removal protects ssh", initial: "legacy ssh", command: "printf '2\\n' | remove_iplimit; setup_update", wantChoice: "false", installed: true, ssh: true},
		{name: "explicit menu re-enable", initial: "legacy", command: "printf '1\\n' | remove_iplimit; install_iplimit; setup_update", wantChoice: "true", setup: true, installed: true, jail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			driver := setupFail2banScriptFixture(t, dir)
			for _, line := range strings.Fields(tc.initial) {
				switch line {
				case "legacy":
					writeFail2banFixture(t, dir, "fail2ban/jail.d/3x-ipl.conf", "[3x-ipl]\nenabled = true\nbantime = 47m\n")
					writeFail2banFixture(t, dir, "installed", "")
					writeFail2banFixture(t, dir, "managed-running", "")
				case "ssh":
					writeFail2banFixture(t, dir, "fail2ban/jail.d/sshd.local", "[sshd]\nenabled = true\n")
					writeFail2banFixture(t, dir, "installed", "")
					writeFail2banFixture(t, dir, "ssh-running", "")
				}
			}
			writeFail2banFixture(t, dir, "env", "XUI_DB_TYPE=postgres\nOTHER_KEY='value with spaces'\n")
			cmd := exec.CommandContext(t.Context(), bash, "-c", driver+"\n"+tc.command)
			for _, e := range os.Environ() {
				if !strings.HasPrefix(e, "XUI_") && !strings.HasPrefix(e, "NONINTERACTIVE=") && !strings.HasPrefix(e, "TEST_DIR=") {
					cmd.Env = append(cmd.Env, e)
				}
			}
			cmd.Env = append(cmd.Env, "TEST_DIR="+dir)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("shell flow: %v\n%s", err, out)
			}
			wantEnv := "XUI_DB_TYPE=postgres\nOTHER_KEY='value with spaces'\n"
			if tc.wantChoice != "" {
				wantEnv += "XUI_ENABLE_FAIL2BAN=" + tc.wantChoice + "\n"
			}
			if env := readScriptState(t, dir, "env"); env != wantEnv {
				t.Errorf("preference file = %q, want stored %s and untouched options", env, tc.wantChoice)
			}
			for path, present := range map[string]bool{"setup": tc.setup, "installed": tc.installed, "fail2ban/jail.d/3x-ipl.conf": tc.jail, "ssh-running": tc.ssh} {
				_, err := os.Stat(filepath.Join(dir, path))
				if (err == nil) != present {
					t.Errorf("%s present = %t, want %t (stat %v)", path, err == nil, present, err)
				}
			}
			if tc.ssh && readScriptState(t, dir, "fail2ban/jail.d/sshd.local") != "[sshd]\nenabled = true\n" {
				t.Errorf("independent SSH jail configuration changed")
			}
			if tc.jail && !tc.setup && !strings.Contains(readScriptState(t, dir, "fail2ban/jail.d/3x-ipl.conf"), "bantime = 47m") {
				t.Error("update rewrote the existing jail's ban duration")
			}
		})
	}
}

func writeFail2banFixture(t *testing.T, dir, path, contents string) {
	t.Helper()
	target := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
}

func setupFail2banScriptFixture(t *testing.T, dir string) string {
	t.Helper()
	driver := `red= green= yellow= plain= release=debian
xui_env_file_path() { printf '%s\n' "$TEST_DIR/env"; }
LOGE() { printf '%s\n' "$*"; }
before_show_menu() { :; }
check_status() { return 1; }
systemctl() {
    if [[ $1 == stop ]]; then rm -f "$TEST_DIR/ssh-running" "$TEST_DIR/managed-running"; fi
}
apt-get() { rm -f "$TEST_DIR/installed"; }
command() {
    if [[ $* == '-v fail2ban-client' ]]; then [[ -f "$TEST_DIR/installed" ]]; else builtin command "$@"; fi
}
fail2ban-client() {
    case "$*" in
        'status 3x-ipl') [[ -f "$TEST_DIR/managed-running" ]] ;;
        'stop 3x-ipl') rm -f "$TEST_DIR/managed-running" ;;
        status)
            local jails=
            [[ ! -f "$TEST_DIR/managed-running" ]] || jails=3x-ipl
            [[ ! -f "$TEST_DIR/ssh-running" ]] || jails="${jails:+$jails, }sshd"
            printf 'Status\n- Jail list:\t%s\n' "$jails" ;;
        -d)
            [[ ! -f "$TEST_DIR/fail2ban/jail.d/3x-ipl.conf" ]] || echo "['add', '3x-ipl', 'auto']"
            [[ ! -f "$TEST_DIR/fail2ban/jail.d/sshd.local" ]] || echo "['add', 'sshd', 'auto']" ;;
        *) return 1 ;;
    esac
}
start_iplimit_jail() { touch "$TEST_DIR/managed-running"; }
setup_fail2ban_iplimit() {
    mkdir -p "$TEST_DIR/fail2ban/jail.d"
    printf '[3x-ipl]\nenabled=true\nbantime=30m\n' > "$TEST_DIR/fail2ban/jail.d/3x-ipl.conf"
    touch "$TEST_DIR/setup" "$TEST_DIR/installed" "$TEST_DIR/managed-running"
}
`
	source, err := os.ReadFile("x-ui.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"confirm", "read_fail2ban_preference", "persist_fail2ban_preference",
		"iplimit_jail_configured", "setup_fail2ban_for_lifecycle", "apply_iplimit_preference",
		"iplimit_remove_conflicts", "iplimit_can_uninstall", "remove_iplimit_jail",
		"install_iplimit", "remove_iplimit",
	} {
		if strings.Contains(string(source), "\n"+name+"() {\n") {
			driver += strings.ReplaceAll(scriptFunction(t, "x-ui.sh", name), "/etc/fail2ban", filepath.Join(dir, "fail2ban"))
		}
	}
	cli := "#!/bin/bash\n" + driver + "\ncase \"$1\" in\n\"setup-fail2ban\")\n"
	if strings.Contains(driver, "\nsetup_fail2ban_for_lifecycle() {\n") {
		cli += "setup_fail2ban_for_lifecycle \"${2:-update}\";;\n"
	} else {
		cli += "setup_fail2ban_iplimit;;\n"
	}
	writeFail2banFixture(t, dir, "x-ui", cli+"esac\n")
	for script, name := range map[string]string{"install.sh": "setup_install", "update.sh": "setup_update"} {
		wrapper := scriptFunction(t, script, "setup_fail2ban")
		wrapper = strings.Replace(wrapper, "setup_fail2ban() {", name+"() {", 1)
		wrapper = strings.ReplaceAll(wrapper, "/usr/bin/x-ui", filepath.Join(dir, "x-ui"))
		driver += wrapper
	}
	driver += "\nexport NONINTERACTIVE=1\n"
	return driver
}

func TestNativeFail2banStockSSHBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("requires bash")
	}
	const stock = "[DEFAULT]\nbackend = auto\n\n[sshd]\nlogpath = %(sshd_log)s\nbackend = %(sshd_backend)s\n"
	const ownJail = "[monitor]\nbackend = polling\nlogpath = /var/log/monitor.log\n"
	const override = "fail2ban/jail.d/3x-ipl-backend.conf"
	for _, tc := range []struct {
		name, release, version, customFile, customContents, want string
	}{
		{name: "Debian 12 journal only", release: "debian", version: "12", want: "[sshd]\nbackend = systemd\n"},
		{name: "Ubuntu 22.04 journal only", release: "ubuntu", version: "2204", want: "[sshd]\nbackend = systemd\n"},
		{name: "file logs available", release: "debian", version: "12", customFile: "auth.log"},
		{name: "operator SSH configuration", release: "ubuntu", version: "2404", customFile: "fail2ban/jail.d/sshd.local", customContents: "[sshd]\nbackend = polling\n"},
		{name: "operator default backend", release: "debian", version: "12", customFile: "fail2ban/jail.local", customContents: "[DEFAULT]\nbackend = polling\n"},
		{name: "legacy persisted override", release: "debian", version: "12", customFile: override, customContents: "[DEFAULT]\nbackend = systemd\n", want: "[DEFAULT]\nbackend = systemd\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFail2banFixture(t, dir, "fail2ban/jail.conf", stock)
			writeFail2banFixture(t, dir, "fail2ban/jail.d/monitor.local", ownJail)
			if tc.customFile != "" {
				writeFail2banFixture(t, dir, tc.customFile, tc.customContents)
			}
			script := scriptFunction(t, "x-ui.sh", "configure_default_sshd_backend")
			script += "\napt-get() { :; }\nrelease=\"$1\" os_version=\"$2\"\nconfigure_default_sshd_backend \"$3/fail2ban\" \"$3/auth.log\""
			out, err := exec.CommandContext(t.Context(), bash, "-c", script, "test", tc.release, tc.version, dir).CombinedOutput()
			if err != nil {
				t.Fatalf("configure default backend: %v, %s", err, out)
			}
			got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(override)))
			if tc.want == "" && !os.IsNotExist(err) {
				t.Fatalf("unexpected override, err=%v: %s", err, got)
			}
			if tc.want != "" && (err != nil || string(got) != tc.want) {
				t.Fatalf("override = %q, want %q, err=%v", got, tc.want, err)
			}
			if readScriptState(t, dir, "fail2ban/jail.conf") != stock || readScriptState(t, dir, "fail2ban/jail.d/monitor.local") != ownJail {
				t.Fatal("stock config or independent jail was rewritten")
			}
		})
	}
}
