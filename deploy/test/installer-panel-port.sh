#!/usr/bin/env bash
# Exercise only installer function definitions, never its installation entry point.
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/../.." && pwd)
installer=${INSTALLER_UNDER_TEST:-"$repo_root/install.sh"}
test_dir=$(mktemp -d)
trap 'rm -rf -- "$test_dir"' EXIT

load_function() {
    local definition
    definition=$(awk -v name="$1" '
        $0 == name "() {" {copy = 1}
        copy {print}
        copy && /^}$/ {exit}
    ' "$installer")
    [[ -n "$definition" ]] || { echo "Missing installer function: $1" >&2; return 1; }
    eval "$definition"
}

load_function is_port_in_use
load_function config_after_install
# The original installer has no selection helpers; its integration tests still run.
if grep -q '^select_panel_port() {' "$installer"; then
    load_function normalize_panel_port
    load_function select_panel_port
fi

# These commands are invoked by the extracted installer functions in a subshell.
# shellcheck disable=SC2317,SC2329
curl() { printf '192.0.2.1\n200\n'; }
# shellcheck disable=SC2317,SC2329
gen_random_string() { printf 'test-credential-%s\n' "$1"; }
# shellcheck disable=SC2317,SC2329
prompt_and_setup_ssl() {
    echo ssl >> "$CASE_DIR/events"
    export SSL_SCHEME=http SSL_HOST=192.0.2.1
}
# shellcheck disable=SC2317,SC2329
write_install_result() { printf '%s\n' "$@" > "$CASE_DIR/result"; }
# shellcheck disable=SC2317,SC2329
read() {
    echo read >> "$CASE_DIR/events"
    [[ "$NONINTERACTIVE" != 1 ]] || return 90
    builtin read -r "$@"
}
# shellcheck disable=SC2317,SC2329
shuf() {
    echo random >> "$CASE_DIR/events"
    local count=0
    [[ ! -f "$CASE_DIR/random-count" ]] || count=$(cat "$CASE_DIR/random-count")
    echo "$((count + 1))" > "$CASE_DIR/random-count"
    [[ "${SHUF_FAIL:-0}" == 0 ]] || return 1
    if ((count > 0)) && [[ -n "${RANDOM_NEXT:-}" ]]; then
        printf '%s\n' "$RANDOM_NEXT"
    else
        printf '54321\n'
    fi
}
# Select the availability backend deterministically, regardless of host packages.
# shellcheck disable=SC2317,SC2329
command() {
    if [[ "${1:-}" == -v && "${2:-}" =~ ^(ss|netstat|lsof)$ ]]; then
        [[ "$2" == "${PORT_BACKEND:-ss}" ]]
    else
        builtin command "$@"
    fi
}
# shellcheck disable=SC2317,SC2329
ss() {
    printf 'State Recv-Q Send-Q Local\ Address:Port Peer\ Address:Port\n'
    if [[ -n "${BUSY_PORT:-}" ]]; then
        printf 'LISTEN 0 128 0.0.0.0:%s 0.0.0.0:*\n' "$BUSY_PORT"
        printf 'LISTEN 0 128 [::]:%s [::]:*\n' "$BUSY_PORT"
    fi
}
# shellcheck disable=SC2317,SC2329
netstat() {
    if [[ -n "${BUSY_PORT:-}" ]]; then
        printf 'tcp 0 0 0.0.0.0:%s 0.0.0.0:* LISTEN\n' "$BUSY_PORT"
        printf 'tcp6 0 0 :::%s :::* LISTEN\n' "$BUSY_PORT"
    fi
}
# shellcheck disable=SC2317,SC2329
lsof() { [[ -n "${BUSY_PORT:-}" && "$*" == *"-iTCP:$BUSY_PORT "* ]]; }

new_case() {
    CASE_DIR=$(mktemp -d "$test_dir/case.XXXXXXXX")
    export CASE_DIR
    : > "$CASE_DIR/events"
    xui_folder="$CASE_DIR/bin"
    mkdir -p "$xui_folder"
    cat > "$xui_folder/x-ui" <<'MOCK'
#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == migrate ]]; then
    echo migrate >> "$CASE_DIR/events"
    exit 0
fi
case "${2:-}" in
    -show) printf 'hasDefaultCredential: true\nwebBasePath: /\nport: 2053\n' ;;
    -getCert) printf 'cert: \n' ;;
    -getApiToken) printf 'apiToken: test-token\n' ;;
    -username)
        printf '%s\n' "$@" > "$CASE_DIR/settings-args"
        case "${SETTING_FAILURE:-}" in
            exit) echo 'controlled settings failure' >&2; exit 23 ;;
            zero) echo 'Database initialization failed: controlled failure'; exit 0 ;;
        esac
        while (($#)); do
            case "$1" in -port) port=$2 ;; esac
            shift
        done
        printf 'Port set successfully: %s\n' "$port"
        [[ "${SETTING_FAILURE:-}" != partial ]] || exit 0
        printf 'Username and password updated successfully\nBase URI path set successfully\n'
        ;;
    *) echo 'Unexpected configuration command' >&2; exit 1 ;;
esac
MOCK
    chmod +x "$xui_folder/x-ui"
    export red='' green='' blue='' yellow='' plain='' release=ubuntu
    export XUI_USERNAME=test-user XUI_PASSWORD=test-pass XUI_WEB_BASE_PATH=test-path
    unset XUI_PANEL_PORT SETTING_FAILURE BUSY_PORT RANDOM_NEXT SHUF_FAIL
    PORT_BACKEND=ss
    NONINTERACTIVE=0
}

run_config() {
    local input=$1
    # The initial newline selects the existing SQLite default. All later input
    # belongs to the port prompts; SSL and persistence are side-effect-free mocks.
    printf '\n%s' "$input" > "$CASE_DIR/input"
    status=0
    (config_after_install) < "$CASE_DIR/input" > "$CASE_DIR/output" 2>&1 || status=$?
}

expect_port() {
    [[ "$status" == 0 ]] || { cat "$CASE_DIR/output"; return 1; }
    local actual
    actual=$(awk 'last == "-port" {print; exit} {last = $0}' "$CASE_DIR/settings-args")
    [[ "$actual" == "$1" ]] || { echo "Port $actual, expected $1"; return 1; }
    grep -Fxq 'Panel Installation Complete!' < <(sed 's/^[[:space:]]*//;s/[[:space:]]*$//' "$CASE_DIR/output")
    [[ -f "$CASE_DIR/result" ]] || return 1
    [[ $(sed -n '3p' "$CASE_DIR/result") == "$1" ]] || return 1
    [[ $(sed -n '1p' "$CASE_DIR/result") == test-user ]] || return 1
    [[ $(sed -n '2p' "$CASE_DIR/result") == test-pass ]] || return 1
    [[ $(sed -n '4p' "$CASE_DIR/result") == test-path ]] || return 1
}

expect_failure() {
    [[ "$status" != 0 ]] || { echo 'Configuration unexpectedly succeeded'; return 1; }
    [[ ! -f "$CASE_DIR/result" ]] || return 1
    ! grep -q 'Panel Installation Complete!\|SSL Certificate Setup\|^ssl$' "$CASE_DIR/output" "$CASE_DIR/events"
}

affirmative() {
    new_case
    run_config "$1"$'\n2053\n'
    expect_port 2053
    ! grep -q '^random$' "$CASE_DIR/events"
}
negative() {
    new_case
    run_config "$1"$'\n'
    expect_port 54321
    [[ $(grep -c '^random$' "$CASE_DIR/events") == 1 ]]
}
invalid_port() {
    new_case
    run_config $'y\n'"$1"$'\n2053\n'
    expect_port 2053
    grep -q 'Invalid port' "$CASE_DIR/output"
    ! grep -q '^random$' "$CASE_DIR/events"
}
valid_port() {
    new_case
    run_config $'y\n'"$1"$'\n'
    expect_port "$2"
}
confirmation_retry() {
    new_case
    run_config $'maybe\nyes please\nYES\n2053\n'
    expect_port 2053
    grep -q 'Please answer yes or no' "$CASE_DIR/output"
    ! grep -q '^random$' "$CASE_DIR/events"
}
busy_port() {
    new_case
    BUSY_PORT=2053
    PORT_BACKEND=$1
    run_config $'y\n2053\n2054\n'
    expect_port 2054
    grep -q 'already in use' "$CASE_DIR/output"
}
random_retry() {
    new_case
    BUSY_PORT=54321 RANDOM_NEXT=54322
    run_config $'no\n'
    expect_port 54322
    [[ $(grep -c '^random$' "$CASE_DIR/events") == 2 ]]
}
random_failure() {
    new_case
    SHUF_FAIL=1
    run_config $'no\n'
    expect_failure
    [[ ! -f "$CASE_DIR/settings-args" ]]
}
random_exhaustion() {
    new_case
    BUSY_PORT=54321
    run_config $'no\n'
    expect_failure
    [[ ! -f "$CASE_DIR/settings-args" ]]
    [[ $(grep -c '^random$' "$CASE_DIR/events") == 100 ]]
}
noninteractive() {
    new_case
    NONINTERACTIVE=1
    case "$1" in unset) ;; *) export XUI_PANEL_PORT=$1 ;; esac
    run_config ''
    expect_port "$2"
    ! grep -q '^read$' "$CASE_DIR/events"
}
noninteractive_invalid() {
    new_case
    NONINTERACTIVE=1
    export XUI_PANEL_PORT=$1
    run_config ''
    expect_failure
    [[ ! -f "$CASE_DIR/settings-args" ]]
    ! grep -q '^read$\|^random$' "$CASE_DIR/events"
}
noninteractive_busy() {
    new_case
    NONINTERACTIVE=1 BUSY_PORT=2053
    export XUI_PANEL_PORT=2053
    run_config ''
    expect_failure
    [[ ! -f "$CASE_DIR/settings-args" ]]
    ! grep -q '^read$\|^random$' "$CASE_DIR/events"
}
settings_failure() {
    new_case
    export SETTING_FAILURE=$1
    run_config $'y\n2053\n'
    expect_failure
    [[ -f "$CASE_DIR/settings-args" ]]
    grep -q 'Failed to configure panel settings' "$CASE_DIR/output"
}
input_eof() {
    new_case
    run_config "$1"
    expect_failure
    [[ ! -f "$CASE_DIR/settings-args" ]]
}

passed=0 failed=0
check() {
    local name=$1
    shift
    # Keep the test body out of an if/|| condition: those disable errexit
    # throughout called functions and can hide a failed intermediate assertion.
    set +e
    (set -e; "$@")
    local result=$?
    set -e
    if [[ "$result" == 0 ]]; then
        printf 'PASS %s\n' "$name"
        passed=$((passed + 1))
    else
        printf 'FAIL %s\n' "$name" >&2
        failed=$((failed + 1))
    fi
}

for answer in y Y yes Yes YES '  yEs  ' $'\tYES\t'; do check "affirmative [$answer]" affirmative "$answer"; done
for answer in n N no No NO '' '  nO  ' $' \t \r '; do check "negative [$answer]" negative "$answer"; done
check 'invalid confirmation retries' confirmation_retry
for port in '' abc 0 -1 65536 00065536 70000 1.5 +2053 '20 53' 0x805 999999999999999999999999 00000 '1+2' 'a[0]'; do
    check "invalid port [$port]" invalid_port "$port"
done
for port in 1 2053 65535; do check "valid port [$port]" valid_port "$port" "$port"; done
check 'leading zeros use decimal' valid_port 00002053 2053
check 'leading zeros containing eight use decimal' valid_port 008080 8080
check 'leading-zero boundary uses decimal' valid_port 065535 65535
for backend in ss netstat lsof; do check "busy port via $backend" busy_port "$backend"; done
check 'busy random port retried' random_retry
check 'random generator failure' random_failure
check 'random search exhaustion' random_exhaustion
check 'noninteractive supplied port' noninteractive 2053 2053
check 'noninteractive decimal normalization' noninteractive 00002053 2053
check 'noninteractive random default' noninteractive unset 54321
for port in '' abc 0 -1 65536 1.5 999999999999999999999; do
    check "noninteractive invalid [$port]" noninteractive_invalid "$port"
done
check 'noninteractive busy port fails' noninteractive_busy
for failure in exit zero partial; do check "settings failure [$failure]" settings_failure "$failure"; done
check 'EOF at confirmation' input_eof ''
check 'EOF at custom port' input_eof $'y\n'

printf '%s passed, %s failed\n' "$passed" "$failed"
[[ "$failed" == 0 ]]
