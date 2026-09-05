#!/usr/bin/env bash
# warp-wireproxy-native.sh
# Полностью неинтерактивный установщик Cloudflare WARP + wireproxy + SOCKS5 для 3x-ui/Xray.
# В режиме --check не запускает apt update/install, чтобы cron/timer были лёгкими.

set -Eeuo pipefail

VERSION="1.2.0"
SOCKS_HOST="127.0.0.1"
SOCKS_PORT="40000"
SCAN_COUNT="50"
USE_CUSTOM_ENDPOINTS="0"
FORCE_REGISTER="0"
CHECK_ONLY="0"
SCANNER="${WARPWP_SCANNER:-native}"
WARPSCOUT_BIN="${WARPWP_WARPSCOUT_BIN:-warpscout}"
WARPSCOUT_JOBS="${WARPWP_WARPSCOUT_JOBS:-6}"
STABILITY_PROBES="${WARPWP_STABILITY_PROBES:-5}"
NODE_ALLOW="${WARPWP_NODE:-}"
NODE_DENY="${WARPWP_AVOID_NODE:-}"
COUNTRY_ALLOW="${WARPWP_COUNTRY:-}"
COUNTRY_DENY="${WARPWP_AVOID_COUNTRY:-}"
POLICY_MODE="${WARPWP_POLICY_MODE:-prefer}"

# Сколько рабочих endpoint'ов достаточно, чтобы прекратить перебор и выбрать
# лучший из найденных. Без этого --deep-scan гонял все 150 полных проверок
# даже когда рабочий нашёлся на первой.
ENOUGH_GOOD="3"
PROBE_TIMEOUT="8"     # curl-таймаут при переборе кандидатов
FINAL_TIMEOUT="15"    # curl-таймаут финальной проверки
PORT_WAIT_TRIES="30"  # 30 x 0.1s = 3s ожидания, пока wireproxy займёт порт

WG_DIR="/etc/wireguard"
WARP_CONF="$WG_DIR/warp.wireproxy.conf"
LEGACY_WARP_CONF="$WG_DIR/warp.conf"
PROXY_CONF="$WG_DIR/proxy.conf"
ACCOUNT_JSON="$WG_DIR/warp-account.json"
PRIVATE_KEY_FILE="$WG_DIR/warp-private.key"
GOOD_ENDPOINTS_FILE="$WG_DIR/warp-endpoints.good"
BAD_ENDPOINTS_FILE="$WG_DIR/warp-endpoints.bad"
SERVICE_FILE="/etc/systemd/system/wireproxy.service"
TEST_URL="https://www.cloudflare.com/cdn-cgi/trace"
RESULT_FILE="/tmp/warp_native_results.$$"
CANDIDATES_FILE="/tmp/warp_native_candidates.$$"
WARPSCOUT_ACCOUNT_FILE=""

BEST_ENDPOINT=""
BEST_TIME=""
BEST_TRACE=""
BEST_COLO=""
BEST_LOC=""
BEST_LOSS=""
BEST_STABLE=""
BEST_SCANNER=""
LAST_POLICY_MATCH="1"
LAST_STABILITY_LOSS="0"
LAST_STABILITY_TORN="0"
LAST_STABILITY_TIME=""
CURRENT_SCANNER="native"
ZAPRET_PORTS="443,2408,1843,1010,500,1701,4500,4443,8443,8095"

WARP_PREFIXES=("162.159.192" "162.159.193" "162.159.194" "162.159.195" "188.114.96" "188.114.97" "188.114.98" "188.114.99" "8.34.146" "8.39.214" "8.39.204" "8.6.112" "8.35.211" "8.39.125" "8.47.69")
WARP_PORTS=(500 854 859 864 878 880 890 891 894 903 908 928 934 939 942 943 945 946 955 968 987 988 1002 1010 1014 1018 1070 1074 1180 1387 1701 1843 2371 2408 2506 3138 3476 3581 3854 4177 4198 4233 4500 5279 5956 7103 7152 7156 7281 7559 8319 8742 8854 8886)
CUSTOM_ENDPOINTS=()

# Диагностика идёт в stderr: иначе command substitution вокруг функций,
# которые логируют, затягивает сообщения в возвращаемое значение.
log()  { printf '\033[1;36m[ИНФО]\033[0m %s\n' "$*" >&2; }
ok()   { printf '\033[1;32m[ОК]\033[0m %s\n' "$*" >&2; }
warn() { printf '\033[1;33m[ВНИМАНИЕ]\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31m[ОШИБКА]\033[0m %s\n' "$*" >&2; }

usage() {
  cat <<EOF_USAGE
Использование:
  bash $0 [опции]

Опции:
  --check                   Только проверить WARP. Если warp=on нет — пересканировать и подменить endpoint.
  --port <порт>             Локальный SOCKS5-порт. По умолчанию: 40000
  --host <хост>             Bind-адрес SOCKS5. По умолчанию: 127.0.0.1
  --scan-count <число>      Сколько случайных endpoint'ов проверить. По умолчанию: 50
  --enough-good <число>     Сколько рабочих endpoint'ов набрать, чтобы прекратить
                            перебор и выбрать лучший из них. По умолчанию: 3
  --scanner <режим>         native, warpscout или auto. По умолчанию: native
  --warpscout-bin <путь>    Путь к бинарнику WARPSCOUT. По умолчанию: warpscout
  --warpscout-jobs <число>  Параллельные туннели WARPSCOUT. По умолчанию: 6
  --stability-probes <N>    Проверок нового endpoint'а, минимум 3. По умолчанию: 5
  --node <коды>             Разрешённые Cloudflare colo, например HEL,ARN
  --avoid-node <коды>       Запрещённые Cloudflare colo, например DME
  --country <коды>          Разрешённые выходные страны, например DE,NL
  --avoid-country <коды>    Запрещённые выходные страны, например RU
  --policy-mode <режим>     prefer (fallback разрешён) или strict
  --ports "список"          Порты для сканирования, через пробел или запятую
  --endpoints "список"      Проверить конкретные endpoint'ы вместо автосканирования
  --force-register          Создать новый WARP-аккаунт, даже если старый уже есть
  --version                 Показать версию
  -h, --help                Показать справку
EOF_USAGE
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --check) CHECK_ONLY="1"; shift ;;
      --port) SOCKS_PORT="${2:-}"; shift 2 ;;
      --host) SOCKS_HOST="${2:-}"; shift 2 ;;
      --scan-count) SCAN_COUNT="${2:-}"; shift 2 ;;
      --ports)
        local raw_ports="${2:-}"
        raw_ports="${raw_ports//,/ }"
        read -r -a WARP_PORTS <<< "$raw_ports"
        shift 2
        ;;
      --endpoints)
        local raw_eps="${2:-}"
        raw_eps="${raw_eps//,/ }"
        read -r -a CUSTOM_ENDPOINTS <<< "$raw_eps"
        USE_CUSTOM_ENDPOINTS="1"
        shift 2
        ;;
      --enough-good) ENOUGH_GOOD="${2:-}"; shift 2 ;;
      --scanner) SCANNER="${2:-}"; shift 2 ;;
      --warpscout-bin) WARPSCOUT_BIN="${2:-}"; shift 2 ;;
      --warpscout-jobs) WARPSCOUT_JOBS="${2:-}"; shift 2 ;;
      --stability-probes) STABILITY_PROBES="${2:-}"; shift 2 ;;
      --node) NODE_ALLOW="${2:-}"; shift 2 ;;
      --avoid-node) NODE_DENY="${2:-}"; shift 2 ;;
      --country) COUNTRY_ALLOW="${2:-}"; shift 2 ;;
      --avoid-country) COUNTRY_DENY="${2:-}"; shift 2 ;;
      --policy-mode) POLICY_MODE="${2:-}"; shift 2 ;;
      --force-register) FORCE_REGISTER="1"; shift ;;
      --version|-v) echo "warp-wireproxy-native.sh v$VERSION"; exit 0 ;;
      -h|--help) usage; exit 0 ;;
      *) err "Неизвестная опция: $1"; usage; exit 1 ;;
    esac
  done

  [[ "$SCAN_COUNT" =~ ^[0-9]+$ && "$SCAN_COUNT" -ge 1 ]] || { err "--scan-count должен быть положительным числом."; exit 1; }
  [[ "$ENOUGH_GOOD" =~ ^[0-9]+$ && "$ENOUGH_GOOD" -ge 1 ]] || { err "--enough-good должен быть положительным числом."; exit 1; }
  [[ "$WARPSCOUT_JOBS" =~ ^[0-9]+$ && "$WARPSCOUT_JOBS" -ge 1 ]] || { err "--warpscout-jobs должен быть положительным числом."; exit 1; }
  [[ "$STABILITY_PROBES" =~ ^[0-9]+$ && "$STABILITY_PROBES" -ge 3 ]] || { err "--stability-probes должен быть числом не меньше 3."; exit 1; }
  case "$SCANNER" in native|warpscout|auto) ;; *) err "--scanner: используй native, warpscout или auto."; exit 1 ;; esac
  case "$POLICY_MODE" in prefer|strict) ;; *) err "--policy-mode: используй prefer или strict."; exit 1 ;; esac
  NODE_ALLOW="$(normalize_code_list "$NODE_ALLOW")"
  NODE_DENY="$(normalize_code_list "$NODE_DENY")"
  COUNTRY_ALLOW="$(normalize_code_list "$COUNTRY_ALLOW")"
  COUNTRY_DENY="$(normalize_code_list "$COUNTRY_DENY")"
}

normalize_code_list() {
  local raw="${1//,/ }" item out=""
  for item in $raw; do
    item="${item^^}"
    [[ "$item" =~ ^[A-Z0-9]{2,4}$ ]] || { err "Некорректный код в policy: $item"; return 1; }
    [[ ",$out," == *",$item,"* ]] && continue
    out="${out:+$out,}$item"
  done
  printf '%s' "$out"
}

list_has_code() {
  local needle="${1^^}" raw="${2//,/ }" item
  for item in $raw; do [[ "${item^^}" == "$needle" ]] && return 0; done
  return 1
}

policy_configured() {
  [[ -n "$NODE_ALLOW$NODE_DENY$COUNTRY_ALLOW$COUNTRY_DENY" ]]
}

endpoint_matches_policy() {
  local colo="${1^^}" country="${2^^}"
  [[ -z "$NODE_ALLOW$NODE_DENY" || -n "$colo" ]] || return 1
  [[ -z "$COUNTRY_ALLOW$COUNTRY_DENY" || -n "$country" ]] || return 1
  [[ -z "$NODE_ALLOW" ]] || list_has_code "$colo" "$NODE_ALLOW" || return 1
  [[ -z "$COUNTRY_ALLOW" ]] || list_has_code "$country" "$COUNTRY_ALLOW" || return 1
  [[ -z "$NODE_DENY" ]] || ! list_has_code "$colo" "$NODE_DENY" || return 1
  [[ -z "$COUNTRY_DENY" ]] || ! list_has_code "$country" "$COUNTRY_DENY" || return 1
  return 0
}

policy_summary() {
  local out=""
  [[ -n "$NODE_ALLOW" ]] && out+=" node=$NODE_ALLOW"
  [[ -n "$NODE_DENY" ]] && out+=" avoid-node=$NODE_DENY"
  [[ -n "$COUNTRY_ALLOW" ]] && out+=" country=$COUNTRY_ALLOW"
  [[ -n "$COUNTRY_DENY" ]] && out+=" avoid-country=$COUNTRY_DENY"
  [[ -n "$out" ]] && printf '%s mode=%s' "${out# }" "$POLICY_MODE" || printf 'none'
}

require_root() { [[ "${EUID}" -eq 0 ]] || { err "Запусти от root."; exit 1; }; }

check_deps_light() {
  local missing=()
  for cmd in curl grep sed awk python3 mktemp systemctl ss sort head cut date ip; do
    command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd")
  done
  if [[ "${#missing[@]}" -gt 0 ]]; then
    err "Не найдены команды для --check: ${missing[*]}"
    err "Запусти обычную установку без --check, чтобы поставить зависимости."
    exit 1
  fi
}

install_deps_apt() {
  apt-get update -y || warn "apt update завершился с ошибкой. Продолжаю: часто причина в сломанном стороннем репозитории."
  DEBIAN_FRONTEND=noninteractive apt-get install -y curl wget ca-certificates grep sed gawk coreutils iproute2 systemd wireguard-tools python3 tar unzip git || { err "Не удалось установить зависимости через apt."; exit 1; }
  if ! command -v awk >/dev/null 2>&1 && command -v gawk >/dev/null 2>&1; then ln -sf "$(command -v gawk)" /usr/local/bin/awk; fi
}
install_deps_dnf() { dnf install -y curl wget ca-certificates grep sed gawk coreutils iproute systemd wireguard-tools python3 tar unzip git; }
install_deps_yum() { yum install -y curl wget ca-certificates grep sed gawk coreutils iproute systemd wireguard-tools python3 tar unzip git; }
install_deps_apk() { apk add --no-cache curl wget ca-certificates grep sed gawk coreutils iproute2 wireguard-tools python3 tar unzip git; }
install_deps() {
  log "Проверяю зависимости..."
  if command -v apt-get >/dev/null 2>&1; then install_deps_apt
  elif command -v dnf >/dev/null 2>&1; then install_deps_dnf
  elif command -v yum >/dev/null 2>&1; then install_deps_yum
  elif command -v apk >/dev/null 2>&1; then install_deps_apk
  else warn "Неизвестный пакетный менеджер. Убедись, что curl, python3, wg и systemctl установлены."; fi
  local missing=()
  for cmd in curl wget grep sed awk python3 wg ip ss systemctl sort head cut uniq tar; do command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd"); done
  [[ "${#missing[@]}" -eq 0 ]] || { err "Не найдены команды: ${missing[*]}"; exit 1; }
  ok "Зависимости готовы."
}

find_wireproxy_bin() {
  if command -v wireproxy >/dev/null 2>&1; then command -v wireproxy; return 0; fi
  for p in /usr/local/bin/wireproxy /usr/bin/wireproxy /opt/bin/wireproxy; do [[ -x "$p" ]] && echo "$p" && return 0; done
  return 1
}

install_wireproxy_from_release() {
  local arch arch_re url tmp tmpdir bin
  arch="$(uname -m)"
  case "$arch" in x86_64|amd64) arch_re="amd64|x86_64" ;; aarch64|arm64) arch_re="arm64|aarch64" ;; armv7l|armv7) arch_re="armv7|arm" ;; *) arch_re="$arch" ;; esac
  log "Пытаюсь скачать wireproxy из GitHub Releases для архитектуры: $arch"
  url="$(python3 - "$arch_re" <<'PY'
import json, re, sys, urllib.request
arch_re = sys.argv[1]
try:
    data = json.load(urllib.request.urlopen('https://api.github.com/repos/pufferffish/wireproxy/releases/latest', timeout=20))
except Exception:
    sys.exit(1)
for a in data.get('assets', []):
    u = a.get('browser_download_url', '')
    s = (a.get('name', '') + ' ' + u).lower()
    if 'linux' in s and re.search(arch_re, s):
        print(u); sys.exit(0)
sys.exit(1)
PY
)" || true
  [[ -z "$url" ]] && return 1
  tmp="/tmp/wireproxy-download.$$"; tmpdir="/tmp/wireproxy-extract.$$"; mkdir -p "$tmpdir"
  curl -fL "$url" -o "$tmp"
  if [[ "$url" == *.zip ]]; then unzip -q "$tmp" -d "$tmpdir"; elif [[ "$url" == *.tar.gz || "$url" == *.tgz ]]; then tar -xzf "$tmp" -C "$tmpdir"; else cp "$tmp" "$tmpdir/wireproxy"; fi
  bin="$(find "$tmpdir" -type f \( -name 'wireproxy' -o -name 'wireproxy-*' \) | head -n1 || true)"
  [[ -n "$bin" ]] || return 1
  install -m 0755 "$bin" /usr/local/bin/wireproxy
  rm -rf "$tmp" "$tmpdir"
  ok "wireproxy установлен: /usr/local/bin/wireproxy"
}
install_wireproxy_from_go() {
  warn "Готового релиза wireproxy не нашёл. Пробую собрать через Go."
  if command -v apt-get >/dev/null 2>&1; then apt-get update -y || true; DEBIAN_FRONTEND=noninteractive apt-get install -y golang-go
  elif command -v dnf >/dev/null 2>&1; then dnf install -y golang
  elif command -v yum >/dev/null 2>&1; then yum install -y golang
  elif command -v apk >/dev/null 2>&1; then apk add --no-cache go; fi
  command -v go >/dev/null 2>&1 || { err "Go не установлен, собрать wireproxy не удалось."; exit 1; }
  GOBIN=/usr/local/bin go install github.com/pufferffish/wireproxy/cmd/wireproxy@latest
  find_wireproxy_bin >/dev/null 2>&1 || { err "wireproxy не появился после go install."; exit 1; }
}
ensure_wireproxy_installed() { find_wireproxy_bin >/dev/null 2>&1 && { ok "wireproxy уже установлен: $(find_wireproxy_bin)"; return 0; }; install_wireproxy_from_release || install_wireproxy_from_go; }

backup_existing() {
  mkdir -p /root/warp-wireproxy-native-backup
  local ts
  ts="$(date +%Y%m%d-%H%M%S)"
  for f in "$WARP_CONF" "$LEGACY_WARP_CONF" "$PROXY_CONF" "$ACCOUNT_JSON" "$SERVICE_FILE" "$GOOD_ENDPOINTS_FILE" "$BAD_ENDPOINTS_FILE"; do
    [[ -f "$f" ]] && cp -a "$f" "/root/warp-wireproxy-native-backup/$(basename "$f").$ts.bak" || true
  done
}

routing_guard_report() {
  echo "--- WARP routing guard ---"
  if ip link show warp >/dev/null 2>&1; then warn "Найден системный интерфейс warp. Он может ломать входящие SSH/443."; else ok "interface warp отсутствует."; fi
  if ip rule show 2>/dev/null | grep -Eq 'lookup (51820|warp)'; then warn "Найдены policy rules WARP:"; ip rule show | grep -E 'lookup (51820|warp)' || true; else ok "policy rules WARP не найдены."; fi
  if ip route show table 51820 2>/dev/null | grep -q .; then warn "Таблица 51820 не пустая:"; ip route show table 51820 || true; else ok "table 51820 пустая/отсутствует."; fi
  for svc in wg-quick@warp wg-quick@wgcf warp-svc; do
    if systemctl is-active --quiet "$svc" 2>/dev/null || systemctl is-enabled --quiet "$svc" 2>/dev/null; then warn "Найден конфликтующий service: $svc"; fi
  done
}

cleanup_system_warp_routes() {
  warn "Проверяю и очищаю системный WARP full-tunnel, если он включён..."
  systemctl disable --now wg-quick@warp wg-quick@wgcf warp-svc 2>/dev/null || true
  ip link del warp 2>/dev/null || true
  while ip rule show 2>/dev/null | grep -Eq 'lookup 51820'; do ip rule del table 51820 2>/dev/null || break; done
  ip route flush table 51820 2>/dev/null || true
  ok "Системные WARP routes очищены. wireproxy SOCKS5 не тронут."
}

register_warp_account() {
  mkdir -p "$WG_DIR"; chmod 700 "$WG_DIR"
  if [[ "$FORCE_REGISTER" != "1" && -f "$ACCOUNT_JSON" && -f "$PRIVATE_KEY_FILE" ]]; then ok "WARP-аккаунт уже есть. Использую существующий $ACCOUNT_JSON"; return 0; fi
  log "Генерирую WireGuard ключи и регистрирую WARP-устройство через API Cloudflare..."
  local private_key public_key tos body tmp
  private_key="$(wg genkey)"; public_key="$(printf '%s' "$private_key" | wg pubkey)"; tos="$(date -u +%Y-%m-%dT%H:%M:%S.000Z)"; tmp="/tmp/warp-register.$$.json"
  printf '%s\n' "$private_key" > "$PRIVATE_KEY_FILE"; chmod 600 "$PRIVATE_KEY_FILE"
  body="$(python3 - "$public_key" "$tos" <<'PY'
import json, sys
pub, tos = sys.argv[1], sys.argv[2]
print(json.dumps({'key': pub, 'install_id': '', 'fcm_token': '', 'tos': tos, 'type': 'Android', 'model': 'PC', 'locale': 'en_US'}))
PY
)"
  curl -fsSL -X POST 'https://api.cloudflareclient.com/v0a2158/reg' -H 'Content-Type: application/json; charset=UTF-8' -H 'User-Agent: okhttp/3.12.1' -H 'CF-Client-Version: a-6.11-2223' --data "$body" > "$tmp"
  python3 - "$tmp" <<'PY'
import json, sys
data=json.load(open(sys.argv[1])); missing=[k for k in ['id','token','config'] if k not in data]
if missing: raise SystemExit('bad response, missing: '+','.join(missing))
PY
  mv "$tmp" "$ACCOUNT_JSON"; chmod 600 "$ACCOUNT_JSON"; ok "WARP-аккаунт зарегистрирован."
}

json_get_config() {
  python3 - "$ACCOUNT_JSON" "$PRIVATE_KEY_FILE" <<'PY'
import json, sys
a=json.load(open(sys.argv[1])); private=open(sys.argv[2]).read().strip(); c=a.get('config',{}); iface=c.get('interface',{}); addresses=iface.get('addresses',{}); peers=c.get('peers',[{}]); peer=peers[0] if peers else {}; endpoint=peer.get('endpoint',{}).get('host') or 'engage.cloudflareclient.com:2408'
print('PRIVATE_KEY='+private)
print('ADDRESS_V4='+addresses.get('v4','172.16.0.2'))
print('ADDRESS_V6='+addresses.get('v6','2606:4700:110:0000:0000:0000:0000:0002'))
print('PEER_PUBLIC_KEY='+peer.get('public_key','bmXOC+F1QSPGQ2ObwTOu6NWKSLW89kykyGw4RrHkGOU='))
print('ENDPOINT='+endpoint)
PY
}

write_configs() {
  local cfg private address_v4 address_v6 peer_public endpoint
  cfg="$(json_get_config)"
  private="$(echo "$cfg" | awk -F= '/^PRIVATE_KEY=/{print substr($0,index($0,"=")+1)}')"
  address_v4="$(echo "$cfg" | awk -F= '/^ADDRESS_V4=/{print substr($0,index($0,"=")+1)}')"
  address_v6="$(echo "$cfg" | awk -F= '/^ADDRESS_V6=/{print substr($0,index($0,"=")+1)}')"
  peer_public="$(echo "$cfg" | awk -F= '/^PEER_PUBLIC_KEY=/{print substr($0,index($0,"=")+1)}')"
  endpoint="$(echo "$cfg" | awk -F= '/^ENDPOINT=/{print substr($0,index($0,"=")+1)}')"
  cat > "$WARP_CONF" <<EOF_WARP
# Internal WARP config mirror for WARP WireProxy Manager.
# Do NOT run this file with wg-quick. Use wireproxy.service and $PROXY_CONF.
[Interface]
PrivateKey = $private
Address = $address_v4/32
Address = $address_v6/128
DNS = 1.1.1.1
MTU = 1280
Table = off
PreUp = /bin/sh -c 'echo "ERROR: WARP WireProxy Manager uses wireproxy SOCKS5 only; do not run wg-quick with this config." >&2; exit 1'

[Peer]
PublicKey = $peer_public
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = $endpoint
PersistentKeepalive = 25
EOF_WARP
  chmod 600 "$WARP_CONF"
  cat > "$PROXY_CONF" <<EOF_PROXY
[Interface]
PrivateKey = $private
Address = $address_v4/32
Address = $address_v6/128
DNS = 1.1.1.1
MTU = 1280

[Peer]
PublicKey = $peer_public
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = $endpoint
PersistentKeepalive = 25

[Socks5]
BindAddress = $SOCKS_HOST:$SOCKS_PORT
EOF_PROXY
  chmod 600 "$PROXY_CONF"
  cat > "$LEGACY_WARP_CONF" <<EOF_LEGACY
# Guard file created by WARP WireProxy Manager.
# This project uses Cloudflare WARP only through wireproxy SOCKS5: $SOCKS_HOST:$SOCKS_PORT
# Do NOT run: wg-quick up warp / systemctl enable --now wg-quick@warp
[Interface]
PrivateKey = $private
Address = $address_v4/32
Address = $address_v6/128
DNS = 1.1.1.1
MTU = 1280
Table = off
PreUp = /bin/sh -c 'echo "ERROR: do not run /etc/wireguard/warp.conf with wg-quick; use wireproxy.service only." >&2; exit 1'

[Peer]
PublicKey = $peer_public
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = $endpoint
PersistentKeepalive = 25
EOF_LEGACY
  chmod 600 "$LEGACY_WARP_CONF"
  ok "Конфиги созданы. Первичный endpoint: $endpoint"
  ok "Защита от случайного wg-quick@warp включена: $LEGACY_WARP_CONF"
}

set_endpoint() {
  local ep="$1"
  [[ -f "$WARP_CONF" ]] && sed -i "s#^Endpoint[[:space:]]*=.*#Endpoint = $ep#I" "$WARP_CONF"
  [[ -f "$LEGACY_WARP_CONF" ]] && sed -i "s#^Endpoint[[:space:]]*=.*#Endpoint = $ep#I" "$LEGACY_WARP_CONF"
  sed -i "s#^Endpoint[[:space:]]*=.*#Endpoint = $ep#I" "$PROXY_CONF"
}
create_service() {
  local bin; bin="$(find_wireproxy_bin)"
  cat > "$SERVICE_FILE" <<EOF_SERVICE
[Unit]
Description=WireProxy for WARP
Documentation=https://github.com/pufferffish/wireproxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$bin -c $PROXY_CONF
Restart=always
RestartSec=3
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF_SERVICE
  systemctl daemon-reload; systemctl enable wireproxy >/dev/null 2>&1 || true; ok "wireproxy.service создан."
}
ensure_service_exists() { [[ -f "$SERVICE_FILE" ]] || systemctl list-unit-files 2>/dev/null | grep -q '^wireproxy\.service' && return 0; find_wireproxy_bin >/dev/null 2>&1 && [[ -f "$PROXY_CONF" ]] && { create_service; return 0; }; return 1; }
# wireproxy занимает порт за доли секунды. Опрос вместо фиксированного
# sleep 2 экономит почти всё время перебора кандидатов.
wait_for_socks_port() { local i; for ((i = 0; i < PORT_WAIT_TRIES; i++)); do if ss -lnt 2>/dev/null | grep -q "$SOCKS_HOST:$SOCKS_PORT"; then return 0; fi; sleep 0.1; done; return 1; }
restart_wireproxy() { ensure_service_exists || { err "wireproxy.service отсутствует, а $PROXY_CONF или бинарник wireproxy не найден."; exit 1; }; systemctl restart wireproxy; wait_for_socks_port || sleep 2; }
check_port() { ss -lntup 2>/dev/null | grep -q "$SOCKS_HOST:$SOCKS_PORT" && ok "SOCKS5 слушает $SOCKS_HOST:$SOCKS_PORT" || { warn "SOCKS5 порт пока не виден. Статус wireproxy:"; systemctl status wireproxy --no-pager -l | head -80 || true; }; }
get_current_endpoint() { grep -i '^Endpoint' "$PROXY_CONF" 2>/dev/null | head -n1 | awk -F= '{gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2}'; }

quick_warp_check() {
  local raw trace endpoint time_total colo loc cache_line _ep _time _colo _loc _ts cached_loss cached_stable cached_scanner
  raw="$(LC_ALL=C curl -m "$PROBE_TIMEOUT" -s -x "socks5h://$SOCKS_HOST:$SOCKS_PORT" -w '\n__TIME_TOTAL__=%{time_total}\n' "$TEST_URL" 2>/dev/null || true)"
  trace="$(printf '%s\n' "$raw" | grep -E '^(ip|colo|loc|warp)=' || true)"
  BEST_TRACE="$trace"
  printf '%s\n' "$trace" | grep -q '^warp=on' || return 1
  endpoint="$(get_current_endpoint || true)"
  colo="$(printf '%s\n' "$trace" | awk -F= '$1=="colo"{print $2; exit}')"
  loc="$(printf '%s\n' "$trace" | awk -F= '$1=="loc"{print $2; exit}')"
  if [[ -n "$endpoint" ]]; then
    # Пишем настоящее время ответа: раньше сюда шёл ноль, из-за чего любой
    # успешный quick-check намертво прибивал текущий endpoint к вершине
    # good-кэша и рейтинг переставал что-либо значить.
    time_total="$(printf '%s\n' "$raw" | grep -m1 '^__TIME_TOTAL__=' | cut -d= -f2- || true)"
    cache_line="$(awk -v ep="$endpoint" -F'\t' '$1==ep{line=$0} END{print line}' "$GOOD_ENDPOINTS_FILE" 2>/dev/null || true)"
    IFS=$'\t' read -r _ep _time _colo _loc _ts cached_loss cached_stable cached_scanner <<< "$cache_line"
    remember_good_endpoint "$endpoint" "${time_total:-9}" "${colo:-unknown}" "${loc:-unknown}" "${cached_loss:-0}" "${cached_stable:-1}" "${cached_scanner:-healthcheck}"
  fi
  if policy_configured && ! endpoint_matches_policy "$colo" "$loc"; then
    LAST_POLICY_MATCH="0"
    warn "Текущий endpoint работает, но не соответствует policy: $(policy_summary)"
    return 1
  fi
  LAST_POLICY_MATCH="1"
  return 0
}
remember_good_endpoint() { local ep="${1:-}" time="${2:-0}" colo="${3:-unknown}" loc="${4:-unknown}" loss="${5:-0}" stable="${6:-1}" scanner="${7:-native}" ts; [[ -z "$ep" ]] && return 0; mkdir -p "$WG_DIR"; ts="$(date +%s)"; touch "$GOOD_ENDPOINTS_FILE" "$BAD_ENDPOINTS_FILE"; awk -v ep="$ep" -F'\t' '$1 != ep {print}' "$GOOD_ENDPOINTS_FILE" > "${GOOD_ENDPOINTS_FILE}.tmp" 2>/dev/null || true; mv "${GOOD_ENDPOINTS_FILE}.tmp" "$GOOD_ENDPOINTS_FILE" 2>/dev/null || true; printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$ep" "$time" "$colo" "$loc" "$ts" "$loss" "$stable" "$scanner" >> "$GOOD_ENDPOINTS_FILE"; LC_ALL=C sort -t $'\t' -k2,2n "$GOOD_ENDPOINTS_FILE" | head -n 30 > "${GOOD_ENDPOINTS_FILE}.tmp" || true; mv "${GOOD_ENDPOINTS_FILE}.tmp" "$GOOD_ENDPOINTS_FILE" 2>/dev/null || true; awk -v ep="$ep" -F'\t' '$1 != ep {print}' "$BAD_ENDPOINTS_FILE" > "${BAD_ENDPOINTS_FILE}.tmp" 2>/dev/null || true; mv "${BAD_ENDPOINTS_FILE}.tmp" "$BAD_ENDPOINTS_FILE" 2>/dev/null || true; }
remember_bad_endpoint() { local ep="${1:-}" ts count; [[ -z "$ep" ]] && return 0; mkdir -p "$WG_DIR"; touch "$BAD_ENDPOINTS_FILE"; ts="$(date +%s)"; count="$(awk -v ep="$ep" -F'\t' '$1 == ep {print $2}' "$BAD_ENDPOINTS_FILE" 2>/dev/null | tail -n1)"; count="${count:-0}"; count=$((count + 1)); awk -v ep="$ep" -F'\t' '$1 != ep {print}' "$BAD_ENDPOINTS_FILE" > "${BAD_ENDPOINTS_FILE}.tmp" 2>/dev/null || true; mv "${BAD_ENDPOINTS_FILE}.tmp" "$BAD_ENDPOINTS_FILE" 2>/dev/null || true; printf '%s\t%s\t%s\n' "$ep" "$count" "$ts" >> "$BAD_ENDPOINTS_FILE"; tail -n 200 "$BAD_ENDPOINTS_FILE" > "${BAD_ENDPOINTS_FILE}.tmp" || true; mv "${BAD_ENDPOINTS_FILE}.tmp" "$BAD_ENDPOINTS_FILE" 2>/dev/null || true; }
is_bad_endpoint() { local ep="${1:-}" now ts count age; [[ -z "$ep" || ! -f "$BAD_ENDPOINTS_FILE" ]] && return 1; now="$(date +%s)"; count="$(awk -v ep="$ep" -F'\t' '$1 == ep {print $2}' "$BAD_ENDPOINTS_FILE" | tail -n1)"; ts="$(awk -v ep="$ep" -F'\t' '$1 == ep {print $3}' "$BAD_ENDPOINTS_FILE" | tail -n1)"; count="${count:-0}"; ts="${ts:-0}"; age=$((now - ts)); [[ "$count" -ge 3 && "$age" -lt 86400 ]]; }
append_candidate() { local ep="${1:-}"; [[ -z "$ep" ]] && return 0; is_bad_endpoint "$ep" && return 0; grep -qxF "$ep" "$CANDIDATES_FILE" 2>/dev/null || echo "$ep" >> "$CANDIDATES_FILE"; }
generate_endpoint_candidates() { : > "$CANDIDATES_FILE"; local current_endpoint ep made attempts prefix last_octet port; current_endpoint="$(get_current_endpoint || true)"; [[ -n "$current_endpoint" ]] && append_candidate "$current_endpoint"; if [[ -f "$GOOD_ENDPOINTS_FILE" ]]; then while IFS=$'\t' read -r ep _rest; do append_candidate "$ep"; done < <(LC_ALL=C sort -t $'\t' -k2,2n "$GOOD_ENDPOINTS_FILE" | head -n 20); fi; if [[ "$USE_CUSTOM_ENDPOINTS" == "1" ]]; then for ep in "${CUSTOM_ENDPOINTS[@]}"; do append_candidate "$ep"; done; return 0; fi; for ep in engage.cloudflareclient.com:2408 162.159.192.244:1843 162.159.195.100:1010 162.159.193.10:2408 188.114.96.10:2408 188.114.97.10:2408; do append_candidate "$ep"; done; made=0; attempts=0; while [[ "$made" -lt "$SCAN_COUNT" && "$attempts" -lt $((SCAN_COUNT * 10 + 200)) ]]; do attempts=$((attempts + 1)); prefix="${WARP_PREFIXES[$((RANDOM % ${#WARP_PREFIXES[@]}))]}"; last_octet="$((RANDOM % 256))"; port="${WARP_PORTS[$((RANDOM % ${#WARP_PORTS[@]}))]}"; ep="${prefix}.${last_octet}:${port}"; if ! grep -qxF "$ep" "$CANDIDATES_FILE" 2>/dev/null && ! is_bad_endpoint "$ep"; then echo "$ep" >> "$CANDIDATES_FILE"; made=$((made + 1)); fi; done; ok "Кандидатов endpoint: $(wc -l < "$CANDIDATES_FILE" | tr -d ' ')"; }

stability_check() {
  local first_time="$1" raw http_code warp time_total i successes=1 tail_failures=0 times
  times="$first_time"
  for ((i = 2; i <= STABILITY_PROBES; i++)); do
    raw="$(LC_ALL=C curl -m "$PROBE_TIMEOUT" -sS -x "socks5h://$SOCKS_HOST:$SOCKS_PORT" -w '\n__TIME_TOTAL__=%{time_total}\n__HTTP_CODE__=%{http_code}\n' "$TEST_URL" 2>/dev/null || true)"
    http_code="$(printf '%s\n' "$raw" | awk -F= '$1=="__HTTP_CODE__"{print $2; exit}')"
    warp="$(printf '%s\n' "$raw" | awk -F= '$1=="warp"{print $2; exit}')"
    time_total="$(printf '%s\n' "$raw" | awk -F= '$1=="__TIME_TOTAL__"{print $2; exit}')"
    if [[ "$http_code" == "200" && "$warp" == "on" && -n "$time_total" ]]; then
      successes=$((successes + 1))
      tail_failures=0
      times+=$'\n'"$time_total"
    else
      tail_failures=$((tail_failures + 1))
    fi
  done
  LAST_STABILITY_LOSS="$(awk -v ok="$successes" -v total="$STABILITY_PROBES" 'BEGIN { printf "%.0f", 100 * (total-ok) / total }')"
  LAST_STABILITY_TIME="$(printf '%s\n' "$times" | awk 'NF {sum+=$1; n++} END {if (n) printf "%.6f", sum/n}')"
  [[ "$tail_failures" -ge 2 ]] && LAST_STABILITY_TORN="1" || LAST_STABILITY_TORN="0"
  [[ "$successes" -ge $((STABILITY_PROBES - 1)) && "$LAST_STABILITY_TORN" == "0" ]]
}

test_endpoint() {
  local ep="$1" trace_file="/tmp/warp_native_trace.$$" rc=0 warp ip colo loc time_total http_code policy
  set_endpoint "$ep"
  restart_wireproxy
  LC_ALL=C curl -m "$PROBE_TIMEOUT" -sS -x "socks5h://$SOCKS_HOST:$SOCKS_PORT" -w '\n__TIME_TOTAL__=%{time_total}\n__HTTP_CODE__=%{http_code}\n' "$TEST_URL" > "$trace_file" 2>/dev/null || rc=$?
  if [[ "$rc" -ne 0 ]]; then
    printf '%s\tFAIL\tcurl_rc=%s\t-\t-\t-\t-\t100\t0\t-\t%s\n' "$ep" "$rc" "$CURRENT_SCANNER" >> "$RESULT_FILE"
    remember_bad_endpoint "$ep"
    rm -f "$trace_file"
    return 1
  fi
  warp="$(grep -m1 '^warp=' "$trace_file" | cut -d= -f2- || true)"
  ip="$(grep -m1 '^ip=' "$trace_file" | cut -d= -f2- || true)"
  colo="$(grep -m1 '^colo=' "$trace_file" | cut -d= -f2- || true)"
  loc="$(grep -m1 '^loc=' "$trace_file" | cut -d= -f2- || true)"
  time_total="$(grep -m1 '^__TIME_TOTAL__=' "$trace_file" | cut -d= -f2- || true)"
  http_code="$(grep -m1 '^__HTTP_CODE__=' "$trace_file" | cut -d= -f2- || true)"
  rm -f "$trace_file"
  if [[ "$http_code" != "200" || "$warp" != "on" || -z "$time_total" ]]; then
    printf '%s\tFAIL\t%s\t%s\t%s\t%s\t%s\t100\t0\t-\t%s\n' "$ep" "${time_total:-9}" "${ip:--}" "${colo:--}" "${loc:--}" "${warp:--}" "$CURRENT_SCANNER" >> "$RESULT_FILE"
    remember_bad_endpoint "$ep"
    return 1
  fi
  if ! stability_check "$time_total"; then
    printf '%s\tUNSTABLE\t%s\t%s\t%s\t%s\t%s\t%s\t0\t-\t%s\n' "$ep" "${LAST_STABILITY_TIME:-$time_total}" "$ip" "$colo" "$loc" "$warp" "$LAST_STABILITY_LOSS" "$CURRENT_SCANNER" >> "$RESULT_FILE"
    remember_bad_endpoint "$ep"
    return 1
  fi
  time_total="${LAST_STABILITY_TIME:-$time_total}"
  LAST_POLICY_MATCH="1"
  policy="MATCH"
  if ! endpoint_matches_policy "$colo" "$loc"; then LAST_POLICY_MATCH="0"; policy="MISMATCH"; fi
  printf '%s\tOK\t%s\t%s\t%s\t%s\t%s\t%s\t1\t%s\t%s\n' "$ep" "$time_total" "$ip" "$colo" "$loc" "$warp" "$LAST_STABILITY_LOSS" "$policy" "$CURRENT_SCANNER" >> "$RESULT_FILE"
  remember_good_endpoint "$ep" "$time_total" "$colo" "$loc" "$LAST_STABILITY_LOSS" "1" "$CURRENT_SCANNER"
  return 0
}

pick_best_line() {
  local result_file="$1" best_line
  best_line="$(awk -F'\t' '$2=="OK" && $10=="MATCH"{print $0}' "$result_file" | LC_ALL=C sort -t $'\t' -k8,8n -k3,3n | head -n1 || true)"
  if [[ -z "$best_line" && "$POLICY_MODE" == "prefer" ]]; then
    best_line="$(awk -F'\t' '$2=="OK"{print $0}' "$result_file" | LC_ALL=C sort -t $'\t' -k8,8n -k3,3n | head -n1 || true)"
  fi
  printf '%s' "$best_line"
}

apply_best_line() {
  local best_line="$1" already_active="${2:-0}"
  BEST_ENDPOINT="$(printf '%s' "$best_line" | awk -F'\t' '{print $1}')"
  BEST_TIME="$(printf '%s' "$best_line" | awk -F'\t' '{print $3}')"
  BEST_COLO="$(printf '%s' "$best_line" | awk -F'\t' '{print $5}')"
  BEST_LOC="$(printf '%s' "$best_line" | awk -F'\t' '{print $6}')"
  BEST_LOSS="$(printf '%s' "$best_line" | awk -F'\t' '{print $8}')"
  BEST_STABLE="$(printf '%s' "$best_line" | awk -F'\t' '{print $9}')"
  BEST_SCANNER="$(printf '%s' "$best_line" | awk -F'\t' '{print $11}')"
  set_endpoint "$BEST_ENDPOINT"
  [[ "$already_active" == "1" ]] || restart_wireproxy
  remember_good_endpoint "$BEST_ENDPOINT" "$BEST_TIME" "$BEST_COLO" "$BEST_LOC" "$BEST_LOSS" "$BEST_STABLE" "$BEST_SCANNER"
  ok "Выбран endpoint: $BEST_ENDPOINT time_total=$BEST_TIME probe_loss=${BEST_LOSS}% colo=$BEST_COLO loc=$BEST_LOC scanner=$BEST_SCANNER"
}

select_best_endpoint_native() {
  CURRENT_SCANNER="native"
  generate_endpoint_candidates
  : > "$RESULT_FILE"
  local ep best_line original_endpoint total good_count=0 tested=0
  # Перебор переписывает Endpoint прямо в боевом конфиге, поэтому запоминаем
  # исходный: если ни один кандидат не взлетит, надо вернуть как было, а не
  # оставить в proxy.conf последний случайный адрес.
  original_endpoint="$(get_current_endpoint || true)"
  total="$(wc -l < "$CANDIDATES_FILE" | tr -d ' ')"
  while IFS= read -r ep; do
    [[ -z "$ep" ]] && continue
    tested=$((tested + 1))
    log "Проверяю $ep"
    if test_endpoint "$ep"; then
      if [[ "$LAST_POLICY_MATCH" == "1" ]]; then
        ok "$ep работает и соответствует policy"
        good_count=$((good_count + 1))
      else
        warn "$ep работает, но не соответствует policy"
      fi
      if [[ "$good_count" -ge "$ENOUGH_GOOD" ]]; then
        log "Набрано рабочих endpoint'ов: $good_count. Останавливаю перебор."
        break
      fi
    else
      warn "$ep не подошёл"
    fi
  done < "$CANDIDATES_FILE"
  if [[ "$tested" -lt "$total" ]]; then log "Проверено кандидатов: $tested из $total (ранняя остановка)."; fi
  echo; echo "=== Результаты проверки ==="
  command -v column >/dev/null 2>&1 && column -t -s $'\t' "$RESULT_FILE" || cat "$RESULT_FILE"
  best_line="$(pick_best_line "$RESULT_FILE")"
  if [[ -n "$best_line" && "$(printf '%s' "$best_line" | awk -F'\t' '{print $10}')" != "MATCH" ]]; then
    warn "Endpoint по policy не найден; использую рабочий fallback (mode=prefer)."
  fi
  if [[ -z "$best_line" ]]; then
    if [[ -n "$original_endpoint" ]]; then
      warn "Рабочий endpoint не найден. Возвращаю прежний: $original_endpoint"
      set_endpoint "$original_endpoint"
      systemctl restart wireproxy 2>/dev/null || true
    fi
    err "Не найден стабильный endpoint с warp=on, подходящий под policy: $(policy_summary)"
    exit 1
  fi
  apply_best_line "$best_line"
}

find_warpscout_bin() {
  if [[ "$WARPSCOUT_BIN" == */* ]]; then [[ -x "$WARPSCOUT_BIN" ]] && printf '%s' "$WARPSCOUT_BIN"; return; fi
  command -v "$WARPSCOUT_BIN" 2>/dev/null || return 1
}

prepare_warpscout_account() {
  [[ -r "$ACCOUNT_JSON" && -r "$PRIVATE_KEY_FILE" ]] || return 1
  [[ -n "$WARPSCOUT_ACCOUNT_FILE" ]] || WARPSCOUT_ACCOUNT_FILE="$(mktemp /tmp/warpwp-warpscout-account.XXXXXX)"
  if (umask 077; python3 - "$ACCOUNT_JSON" "$PRIVATE_KEY_FILE" > "$WARPSCOUT_ACCOUNT_FILE") <<'PY'
import json, sys
account_path, private_path = sys.argv[1:]
with open(account_path, encoding='utf-8') as fh:
    account = json.load(fh)
with open(private_path, encoding='utf-8') as fh:
    private_key = fh.read().strip()
peers = account.get('config', {}).get('peers', [])
peer_public_key = peers[0].get('public_key', '') if peers else ''
if not private_key or not peer_public_key:
    raise SystemExit('WARP account has no private or peer public key')
json.dump({
    'private_key': private_key,
    'peer_public_key': peer_public_key,
}, sys.stdout)
PY
  then
    chmod 600 "$WARPSCOUT_ACCOUNT_FILE"
    return 0
  fi
  rm -f "$WARPSCOUT_ACCOUNT_FILE"
  WARPSCOUT_ACCOUNT_FILE=""
  return 1
}

valid_scanned_endpoint() {
  local ep="$1" host port
  if [[ "$ep" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]{1,5}$ ]]; then
    host="${ep%:*}"
  elif [[ "$ep" =~ ^\[[0-9A-Fa-f:]+\]:[0-9]{1,5}$ ]]; then
    host="${ep%%]:*}"; host="${host:1}"
  else
    return 1
  fi
  port="${ep##*:}"; port="${port%]}"
  [[ "$port" -ge 1 && "$port" -le 65535 ]] || return 1
  python3 -c 'import ipaddress, sys; ipaddress.ip_address(sys.argv[1])' "$host" >/dev/null 2>&1
}

select_best_endpoint_warpscout() {
  local bin sample ping_count output endpoint original_endpoint best_line
  local -a args
  [[ "$USE_CUSTOM_ENDPOINTS" == "0" ]] || { warn "WARPSCOUT mode не поддерживает список endpoint:port; использую native scanner."; return 1; }
  bin="$(find_warpscout_bin)" || { warn "WARPSCOUT не найден: $WARPSCOUT_BIN"; return 1; }
  prepare_warpscout_account || { warn "Не удалось подготовить временный WARPSCOUT account из текущего WARP account."; return 1; }
  sample=$(((SCAN_COUNT + 14) / 15)); [[ "$sample" -gt 256 ]] && sample=256
  ping_count="$STABILITY_PROBES"; [[ "$ping_count" -lt 5 ]] && ping_count=5
  args=(scan -p wg -a "$WARPSCOUT_ACCOUNT_FILE" -n "$sample" -jt "$WARPSCOUT_JOBS" -t "$PROBE_TIMEOUT" -tun-ping-count "$ping_count" -plain -best -no-report)
  [[ -n "$NODE_ALLOW" ]] && args+=(-node "$NODE_ALLOW")
  original_endpoint="$(get_current_endpoint || true)"
  log "WARPSCOUT discovery: sample=$sample jobs=$WARPSCOUT_JOBS policy=$(policy_summary)"
  if ! output="$("$bin" "${args[@]}")"; then
    warn "WARPSCOUT не нашёл подходящий endpoint."
    return 1
  fi
  endpoint="$(printf '%s\n' "$output" | awk 'NF {line=$0} END {print line}')"
  valid_scanned_endpoint "$endpoint" || { warn "WARPSCOUT вернул некорректный endpoint: $endpoint"; return 1; }
  : > "$RESULT_FILE"
  CURRENT_SCANNER="warpscout"
  log "Финально проверяю найденный WARPSCOUT endpoint через wireproxy: $endpoint"
  if ! test_endpoint "$endpoint"; then
    warn "WARPSCOUT endpoint не прошёл финальную проверку Manager."
    if [[ -n "$original_endpoint" ]]; then set_endpoint "$original_endpoint"; systemctl restart wireproxy 2>/dev/null || true; fi
    return 1
  fi
  if [[ "$LAST_POLICY_MATCH" != "1" ]]; then
    if [[ "$POLICY_MODE" == "prefer" && "$SCANNER" == "warpscout" ]]; then
      warn "WARPSCOUT endpoint не соответствует policy; принимаю fallback в mode=prefer."
    else
      warn "WARPSCOUT endpoint не соответствует policy; ищу другой через native scanner."
      if [[ -n "$original_endpoint" ]]; then set_endpoint "$original_endpoint"; systemctl restart wireproxy 2>/dev/null || true; fi
      return 1
    fi
  fi
  best_line="$(awk -F'\t' '$2=="OK"{print $0; exit}' "$RESULT_FILE")"
  [[ -n "$best_line" ]] || return 1
  apply_best_line "$best_line" "1"
  return 0
}

select_best_endpoint() {
  case "$SCANNER" in
    native) select_best_endpoint_native ;;
    warpscout)
      select_best_endpoint_warpscout || { err "WARPSCOUT scanner не смог выбрать endpoint."; exit 1; }
      ;;
    auto)
      if select_best_endpoint_warpscout; then return 0; fi
      warn "Перехожу к встроенному native scanner."
      select_best_endpoint_native
      ;;
  esac
}
final_check() { BEST_TRACE="$(curl -m "$FINAL_TIMEOUT" -s -x "socks5h://$SOCKS_HOST:$SOCKS_PORT" "$TEST_URL" | grep -E 'ip=|colo=|loc=|warp=' || true)"; echo "$BEST_TRACE"; echo "$BEST_TRACE" | grep -q '^warp=on' || { err "Финальная проверка не показала warp=on."; systemctl status wireproxy --no-pager -l | head -80 || true; exit 1; }; ok "WARP работает: warp=on"; }

run_check_and_repair() { cleanup_system_warp_routes; log "Режим проверки: проверяю текущий WARP без переустановки и без apt update."; [[ -f "$PROXY_CONF" ]] || { err "Не найден $PROXY_CONF. Сначала запусти обычную установку без --check."; exit 1; }; find_wireproxy_bin >/dev/null 2>&1 || { err "wireproxy не найден. Сначала запусти обычную установку без --check."; exit 1; }; ensure_service_exists || create_service; systemctl restart wireproxy || true; sleep 2; check_port; if quick_warp_check; then ok "WARP живой, endpoint менять не нужно."; echo "$BEST_TRACE"; echo; echo "Текущий endpoint: $(get_current_endpoint)"; echo "Кэш good endpoint'ов: $GOOD_ENDPOINTS_FILE"; exit 0; fi; warn "WARP не отвечает или нет warp=on. Запускаю быстрый перескан endpoint'ов..."; backup_existing; select_best_endpoint; final_check; echo; ok "Endpoint был автоматически заменён на рабочий: $BEST_ENDPOINT"; print_result; }

print_result() { local endpoint_port; endpoint_port="${BEST_ENDPOINT##*:}"; cat <<EOF_RESULT

============================================================
ГОТОВО
============================================================

Выбранный WARP endpoint:
  $BEST_ENDPOINT
  scanner=$BEST_SCANNER time_total=$BEST_TIME probe_loss=${BEST_LOSS}% stable=$BEST_STABLE
  colo=$BEST_COLO loc=$BEST_LOC
  policy=$(policy_summary)

Локальный SOCKS5:
  socks5://$SOCKS_HOST:$SOCKS_PORT

Cloudflare trace:
$(echo "$BEST_TRACE" | sed 's/^/  /')

------------------------------------------------------------
3x-ui / Xray outbounds
------------------------------------------------------------

{
  "tag": "WARP-socks5",
  "protocol": "socks",
  "settings": {"servers": [{"address": "$SOCKS_HOST", "port": $SOCKS_PORT}]}
},
{
  "tag": "WARP",
  "protocol": "freedom",
  "settings": {"domainStrategy": "UseIPv4"},
  "proxySettings": {"tag": "WARP-socks5"}
}

Важно: в routing указывай "WARP", не "WARP-socks5".
Не запускай wg-quick@warp: этот проект использует WARP только через wireproxy SOCKS5.

zapret4rocket:
  NFQWS_PORTS_UDP=$ZAPRET_PORTS
  endpoint UDP port: $endpoint_port

Проверить вручную:
  curl -m 10 -s -x socks5h://$SOCKS_HOST:$SOCKS_PORT https://www.cloudflare.com/cdn-cgi/trace | grep -E 'ip=|colo=|loc=|warp='

EOF_RESULT
}
cleanup() { rm -f "$RESULT_FILE" "$CANDIDATES_FILE" "$WARPSCOUT_ACCOUNT_FILE" /tmp/warp_native_trace.$$ 2>/dev/null || true; }
main() { trap cleanup EXIT; parse_args "$@"; require_root; if [[ "$CHECK_ONLY" == "1" ]]; then check_deps_light; run_check_and_repair; exit 0; fi; install_deps; cleanup_system_warp_routes; ensure_wireproxy_installed; backup_existing; register_warp_account; write_configs; create_service; restart_wireproxy; check_port; select_best_endpoint; final_check; routing_guard_report; print_result; }
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
