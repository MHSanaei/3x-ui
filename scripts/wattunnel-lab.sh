#!/usr/bin/env bash
# Run only inside a disposable Ubuntu 24.04 VM. Every firewall rule lives in a new namespace.
set -euo pipefail
[[ $(uname -s) == Linux && $EUID == 0 ]] || { echo 'Linux root in a disposable VM is required' >&2; exit 1; }
: "${WATTUN_E2E_BINARY:?Set the absolute path to the verified wattun binary}"
for tool in ip tc nft python3 curl; do command -v "$tool" >/dev/null; done
[[ $WATTUN_E2E_BINARY = /* && -x $WATTUN_E2E_BINARY ]] || exit 1
out=${WATTUN_LAB_OUTPUT:-$(mktemp -d /tmp/wattunnel-lab.XXXXXX)}
mkdir -p "$out"
chmod 700 "$out"
left="wat-c-$$"
right="wat-s-$$"
read -r -a losses <<< "${WATTUN_LAB_LOSSES:-0 1 5 10}"
for loss in "${losses[@]}"; do
  case "$loss" in 0|1|5|10) ;; *) echo 'Loss must be 0, 1, 5 or 10 percent' >&2; exit 1 ;; esac
done
client_pid=''
server_pid=''
stop_side() { local pid=$1; [[ -z $pid ]] || { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; }; }
cleanup() {
  for ns in "$left" "$right"; do
    while read -r pid; do [[ -z $pid ]] || kill "$pid" 2>/dev/null || true; done < <(ip netns pids "$ns" 2>/dev/null)
    ip netns del "$ns" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM
ip netns add "$left"
ip netns add "$right"
ip link add "wc$$" type veth peer name "ws$$"
ip link set "wc$$" netns "$left"
ip link set "ws$$" netns "$right"
ip -n "$left" link set "wc$$" name eth0
ip -n "$right" link set "ws$$" name eth0
for ns in "$left" "$right"; do
  ip -n "$ns" link set lo up
  ip -n "$ns" link set eth0 up
done
ip -n "$left" addr add 192.0.2.1/24 dev eth0
ip -n "$right" addr add 192.0.2.2/24 dev eth0
python3 -c 'import secrets; print(secrets.token_hex(32))' > "$out/key"
chmod 600 "$out/key"
ip netns exec "$right" python3 -u -c '
import socket,threading
s=socket.socket(); s.bind(("127.0.0.1",9000)); s.listen()
def echo(c):
 try:
  with c:
   while b:=c.recv(65536): c.sendall(b)
 except OSError: pass
while True:
 c,_=s.accept(); threading.Thread(target=echo,args=(c,),daemon=True).start()
' > "$out/echo.log" 2>&1 &
start_server() { ip netns exec "$right" "$WATTUN_E2E_BINARY" -config "$out/server.yml" >> "$out/server.log" 2>&1 & server_pid=$!; }
start_client() { ip netns exec "$left" "$WATTUN_E2E_BINARY" -config "$out/client.yml" >> "$out/client.log" 2>&1 & client_pid=$!; }
ready() {
  for _ in {1..60}; do
    if ip netns exec "$left" python3 -c '
import json, socket, urllib.request
with urllib.request.urlopen("http://127.0.0.1:19091/stats", timeout=1) as response:
    assert json.load(response).get("alive")
with socket.create_connection(("127.0.0.1",19000), timeout=1) as conn:
    conn.sendall(b"ready")
    data = bytearray()
    while len(data) < 5:
        chunk = conn.recv(5-len(data))
        assert chunk
        data.extend(chunk)
    assert data == b"ready"
' 2>/dev/null; then return 0; fi
    sleep 1
  done
  echo 'authenticated readiness timed out' >&2; return 1
}
probe() {
 ip netns exec "$left" python3 "$(dirname "$0")/wattunnel-lab-probe.py" "$1" "$2" "$client_pid" "$server_pid" >> "$out/results.jsonl"
}
for carrier in tcp udp tls wss icmp; do
  for role in server client; do
    metrics=19090; ports=9000/tcp; peer=192.0.2.1; listen='192.0.2.2:4443'
    if [[ $role == client ]]; then metrics=19091; ports=19000:9000/tcp; peer=192.0.2.2; listen='192.0.2.1:4443'; fi
    [[ $carrier == icmp ]] || peer="$peer:4443"
    cat > "$out/$role.yml" <<CFG
mode: $role
carrier: $carrier
peer: $peer
listen: $listen
key-file: $out/key
metrics: 127.0.0.1:$metrics
bind: 127.0.0.1
conns: 1
ports:
  - $ports
CFG
  done
  chmod 600 "$out/"*.yml
  start_server; start_client; ready
  for loss in "${losses[@]}"; do
    for ns in "$left" "$right"; do ip netns exec "$ns" tc qdisc replace dev eth0 root netem delay 20ms loss "$loss%"; done
    probe "$carrier" "loss-$loss"
  done
  for ns in "$left" "$right"; do ip netns exec "$ns" tc qdisc del dev eth0 root; done
  ip netns exec "$right" nft -f - <<'NFT'
table inet wattunnel_lab {
 chain input { type filter hook input priority 0; policy accept;
  iifname "eth0" tcp dport 4443 drop
  iifname "eth0" udp dport 4443 drop
 }
}
NFT
  probe "$carrier" blocked-tcp-udp
  ip netns exec "$right" nft delete table inet wattunnel_lab
  ready; probe "$carrier" restored
  stop_side "$server_pid"; start_server; ready; probe "$carrier" server-restart
  stop_side "$client_pid"; start_client; ready; probe "$carrier" client-restart
  ip -n "$left" link set eth0 down
  probe "$carrier" disconnected
  ip -n "$left" link set eth0 up
  ready; probe "$carrier" reconnected
  stop_side "$client_pid"; stop_side "$server_pid"
  client_pid=''; server_pid=''
done
printf 'Results: %s/results.jsonl\n' "$out"
