"""Bounded echo throughput and engine metrics for the isolated WatTunnel lab."""
import json
import os
import socket
import sys
import time
import urllib.request

carrier, scenario, client_pid, server_pid = sys.argv[1:]
result = {"carrier": carrier, "scenario": scenario}
clock = os.sysconf("SC_CLK_TCK")

def cpu_seconds(pid):
    with open(f"/proc/{pid}/stat", encoding="utf-8") as stat:
        fields = stat.read().split()
    return (int(fields[13]) + int(fields[14])) / clock

started = time.monotonic()
client_cpu, server_cpu = cpu_seconds(client_pid), cpu_seconds(server_pid)
transferred = 0
try:
    with socket.create_connection(("127.0.0.1", 19000), timeout=5) as conn:
        conn.settimeout(5)
        payload = b"w" * 65536
        for _ in range(32):
            if time.monotonic() - started > 30:
                raise TimeoutError("30 second probe budget")
            conn.sendall(payload)
            reply = bytearray()
            while len(reply) < len(payload):
                chunk = conn.recv(len(payload) - len(reply))
                if not chunk:
                    raise ConnectionError("unexpected EOF")
                reply.extend(chunk)
            if reply != payload:
                raise ValueError("echo payload mismatch")
            transferred += len(reply)
    result["success"] = True
except (OSError, ValueError) as error:
    result.update(success=False, error=str(error))
result.update(seconds=time.monotonic() - started, bytes=transferred, client_cpu_seconds=cpu_seconds(client_pid) - client_cpu, server_cpu_seconds=cpu_seconds(server_pid) - server_cpu)
result["mbps"] = transferred * 8 / result["seconds"] / 1e6
try:
    with urllib.request.urlopen("http://127.0.0.1:19091/stats", timeout=1) as response:
        stats = json.load(response)
    result["engine"] = {key: stats.get(key) for key in ("alive", "srtt_ms", "loss_pct", "send_errors", "bytes_in_total", "bytes_out_total")}
except (OSError, ValueError) as error:
    result["metrics_error"] = str(error)
print(json.dumps(result), flush=True)
