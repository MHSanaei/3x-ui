#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
test_root=$(mktemp -d)
trap 'rm -rf -- "$test_root"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

extract_function() {
    local script="$1"
    local function_name="$2"
    awk -v signature="${function_name}() {" '
        $0 == signature { found = 1 }
        found { print }
        found && $0 == "}" { exit }
    ' "$script"
}

load_functions() {
    local script="$1"
    shift
    local source_file="${test_root}/$(basename "$script").functions.$$"
    : > "$source_file"
    local function_name
    for function_name in "$@"; do
        extract_function "$script" "$function_name" >> "$source_file"
    done
    # shellcheck disable=SC1090
    source "$source_file"
}

assert_file_equals() {
    local expected="$1"
    local file="$2"
    [[ -f "$file" ]] || fail "missing file: $file"
    [[ "$(< "$file")" == "$expected" ]] || fail "file changed unexpectedly: $file"
}

test_noninteractive_update_preserves_empty_tls() (
    load_functions "$repo_root/update.sh" panel_url_host_from_cert config_after_update

    local case_root="${test_root}/noninteractive"
    mkdir -p "${case_root}/bin"
    cat > "${case_root}/bin/x-ui" <<'FAKE_XUI'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${FAKE_XUI_LOG}"
case "$*" in
    "setting -show true")
        printf 'port: 54321\nwebBasePath: existing-panel-path\n'
        ;;
    "setting -getCert true")
        printf 'cert: %s\nkey: %s\n' "${FAKE_CERT:-}" "${FAKE_KEY:-}"
        ;;
    "migrate") ;;
    *) exit 0 ;;
esac
FAKE_XUI
    chmod +x "${case_root}/bin/x-ui"

    xui_folder="${case_root}/bin"
    export FAKE_XUI_LOG="${case_root}/x-ui.log"
    : > "$FAKE_XUI_LOG"
    NONINTERACTIVE=1
    XUI_SERVER_IP="198.51.100.10"
    green=""
    yellow=""
    red=""
    plain=""
    curl() { printf '\n503\n'; }
    gen_random_string() { printf 'unused-random-path'; }
    systemctl() { return 0; }
    rc-service() { return 0; }
    prompt_and_setup_ssl() {
        touch "${case_root}/unexpected-acme-prompt"
        return 1
    }

    local output
    output=$(config_after_update)
    [[ ! -e "${case_root}/unexpected-acme-prompt" ]] || fail "non-interactive update entered TLS setup"
    grep -Fq 'Access URL: http://198.51.100.10:54321/existing-panel-path' <<< "$output" || \
        fail "non-interactive update did not report the preserved HTTP URL"
    grep -Fq 'SSL Certificate: not configured; panel remains HTTP-only' <<< "$output" || \
        fail "non-interactive update reported a false TLS result"

    export FAKE_CERT='/root/cert/panel/panel.example/fullchain.pem'
    export FAKE_KEY='/root/cert/panel/panel.example/privkey.pem'
    output=$(config_after_update)
    [[ ! -e "${case_root}/unexpected-acme-prompt" ]] || fail "non-interactive update entered TLS setup"
    grep -Fq 'Access URL: https://panel.example:54321/existing-panel-path' <<< "$output" || \
        fail "non-interactive update did not preserve the configured TLS URL"
    if grep -Eq '^cert ' "$FAKE_XUI_LOG"; then
        fail "non-interactive update changed the configured certificate paths"
    fi
)

test_failed_acme_preserves_existing_pair() (
    local script="$1"
    local case_name="$2"
    load_functions "$script" acme_executable panel_certificate_dir setup_ip_certificate

    local case_root="${test_root}/${case_name}"
    local ip="198.51.100.24"
    export HOME="${case_root}/home"
    export XUI_ACME_HOME="${HOME}/.acme.sh"
    export XUI_CERT_ROOT="${case_root}/cert"
    export XUI_ACME_HTTP_PORT=80
    mkdir -p "$XUI_ACME_HOME" "${XUI_ACME_HOME}/${ip}_ecc"
    cat > "${XUI_ACME_HOME}/acme.sh" <<'FAKE_ACME'
#!/usr/bin/env bash
for arg in "$@"; do
    [[ "$arg" == "--issue" ]] && exit 42
done
exit 0
FAKE_ACME
    chmod +x "${XUI_ACME_HOME}/acme.sh"

    local target="${XUI_CERT_ROOT}/panel/ip-${ip}"
    local shared="${XUI_CERT_ROOT}/ip"
    mkdir -p "$target" "$shared"
    printf 'existing-panel-certificate' > "${target}/fullchain.pem"
    printf 'existing-panel-private-key' > "${target}/privkey.pem"
    printf 'existing-shared-certificate' > "${shared}/fullchain.pem"
    printf 'existing-acme-record' > "${XUI_ACME_HOME}/${ip}_ecc/account.conf"

    NONINTERACTIVE=1
    xui_folder="${case_root}/bin"
    green=""
    yellow=""
    red=""
    plain=""
    is_ipv4() { [[ "$1" == "$ip" ]]; }
    is_ipv6() { return 1; }
    is_port_in_use() { return 1; }
    prompt_or_default() { printf -v "$1" '%s' "${4:+${!4}}"; }

    local output
    if output=$(setup_ip_certificate "$ip" "" 2>&1); then
        fail "${case_name}: simulated ACME failure unexpectedly succeeded"
    fi
    grep -Fq 'Existing certificate files and ACME records were left untouched' <<< "$output" || \
        fail "${case_name}: failure did not report preservation"
    assert_file_equals 'existing-panel-certificate' "${target}/fullchain.pem"
    assert_file_equals 'existing-panel-private-key' "${target}/privkey.pem"
    assert_file_equals 'existing-shared-certificate' "${shared}/fullchain.pem"
    assert_file_equals 'existing-acme-record' "${XUI_ACME_HOME}/${ip}_ecc/account.conf"
)

test_xui_failure_paths_are_non_destructive() {
    local function_name body
    for function_name in ssl_cert_issue_for_ip ssl_cert_issue ssl_cert_issue_CF; do
        body=$(extract_function "$repo_root/x-ui.sh" "$function_name")
        grep -Fq 'install_certificate_staged' <<< "$body" || \
            fail "x-ui.sh:${function_name} does not use staged certificate installation"
        if grep -Eq 'rm -rf .*acme|rm -rf .*certPath' <<< "$body"; then
            fail "x-ui.sh:${function_name} deletes certificate or ACME state in an error-capable path"
        fi
    done
}

test_existing_acme_target_is_preserved() (
    local script="$1"
    local case_name="$2"
    load_functions "$script" acme_executable acme_config_value acme_install_target_dir certificate_target_dir

    local case_root="${test_root}/existing-target-${case_name}"
    local identifier="legacy.example"
    export HOME="${case_root}/home"
    export XUI_ACME_HOME="${HOME}/.acme.sh"
    export XUI_CERT_ROOT="${case_root}/cert"
    local legacy_dir="${XUI_CERT_ROOT}/${identifier}"
    local preferred_dir="${XUI_CERT_ROOT}/panel/${identifier}"
    mkdir -p "${XUI_ACME_HOME}/${identifier}_ecc" "$legacy_dir"
    printf 'legacy-certificate' > "${legacy_dir}/fullchain.pem"
    printf 'legacy-private-key' > "${legacy_dir}/privkey.pem"
    cat > "${XUI_ACME_HOME}/${identifier}_ecc/${identifier}.conf" <<EOF
Le_RealFullChainPath='${legacy_dir}/fullchain.pem'
Le_RealKeyPath='${legacy_dir}/privkey.pem'
EOF
    cat > "${XUI_ACME_HOME}/acme.sh" <<EOF
#!/usr/bin/env bash
[[ "\${1:-}" == "--list" ]] && printf 'Main_Domain\n${identifier}\n'
exit 0
EOF
    chmod +x "${XUI_ACME_HOME}/acme.sh"

    local actual
    actual=$(certificate_target_dir "$identifier" "$preferred_dir" "$legacy_dir")
    [[ "$actual" == "$legacy_dir" ]] || \
        fail "${case_name}: existing ACME target changed from ${legacy_dir} to ${actual}"

    rm -f "${XUI_ACME_HOME}/${identifier}_ecc/${identifier}.conf"
    actual=$(certificate_target_dir "$identifier" "$preferred_dir" "$legacy_dir")
    [[ "$actual" == "$legacy_dir" ]] || \
        fail "${case_name}: tracked legacy certificate moved without Le_Real paths"

    actual=$(certificate_target_dir fresh.example \
        "${XUI_CERT_ROOT}/panel/fresh.example" "${XUI_CERT_ROOT}/fresh.example")
    [[ "$actual" == "${XUI_CERT_ROOT}/panel/fresh.example" ]] || \
        fail "${case_name}: a new panel certificate did not use the isolated directory"
)

test_xui_discovers_legacy_and_panel_certificates() (
    load_functions "$repo_root/x-ui.sh" \
        acme_executable acme_config_value acme_install_target_dir \
        panel_certificate_acme_ids list_panel_certificate_names panel_certificate_path

    local case_root="${test_root}/certificate-discovery"
    export HOME="${case_root}/home"
    export XUI_ACME_HOME="${HOME}/.acme.sh"
    export XUI_CERT_ROOT="${case_root}/cert"
    mkdir -p "$XUI_ACME_HOME"
    cat > "${XUI_ACME_HOME}/acme.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "${XUI_ACME_HOME}/acme.sh"
    mkdir -p \
        "${XUI_CERT_ROOT}/legacy.example" \
        "${XUI_CERT_ROOT}/ip" \
        "${XUI_CERT_ROOT}/panel/legacy.example" \
        "${XUI_CERT_ROOT}/panel/new.example"
    printf legacy > "${XUI_CERT_ROOT}/legacy.example/fullchain.pem"
    printf legacy > "${XUI_CERT_ROOT}/legacy.example/privkey.pem"
    printf legacy > "${XUI_CERT_ROOT}/ip/fullchain.pem"
    printf legacy > "${XUI_CERT_ROOT}/ip/privkey.pem"
    printf duplicate > "${XUI_CERT_ROOT}/panel/legacy.example/fullchain.pem"
    printf duplicate > "${XUI_CERT_ROOT}/panel/legacy.example/privkey.pem"
    printf new > "${XUI_CERT_ROOT}/panel/new.example/fullchain.pem"
    printf new > "${XUI_CERT_ROOT}/panel/new.example/privkey.pem"
    mkdir -p "${XUI_ACME_HOME}/legacy.example_ecc"
    cat > "${XUI_ACME_HOME}/legacy.example_ecc/legacy.example.conf" <<EOF
Le_RealFullChainPath='${XUI_CERT_ROOT}/legacy.example/fullchain.pem'
Le_RealKeyPath='${XUI_CERT_ROOT}/legacy.example/privkey.pem'
EOF

    local names
    names=$(list_panel_certificate_names)
    [[ "$names" == $'ip\nlegacy.example\nnew.example' ]] || \
        fail "x-ui.sh: certificate discovery omitted legacy or panel directories: ${names}"
    [[ "$(panel_certificate_path legacy.example)" == "${XUI_CERT_ROOT}/legacy.example" ]] || \
        fail "x-ui.sh: legacy domain path was not resolved"
    [[ "$(panel_certificate_path new.example)" == "${XUI_CERT_ROOT}/panel/new.example" ]] || \
        fail "x-ui.sh: panel domain path was not resolved"
)

test_noninteractive_update_preserves_empty_tls
test_failed_acme_preserves_existing_pair "$repo_root/update.sh" update
test_failed_acme_preserves_existing_pair "$repo_root/install.sh" install
test_xui_failure_paths_are_non_destructive
test_existing_acme_target_is_preserved "$repo_root/update.sh" update
test_existing_acme_target_is_preserved "$repo_root/install.sh" install
test_existing_acme_target_is_preserved "$repo_root/x-ui.sh" x-ui
test_xui_discovers_legacy_and_panel_certificates

echo "TLS update safety regression tests passed"
