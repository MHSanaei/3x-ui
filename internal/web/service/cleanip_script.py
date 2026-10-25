#!/usr/bin/env python3
# 3x-ui clean IP test. Run it with the VPN OFF, on the internet (ISP) you want IPs for.
# Standard library only (Python 3.8+). The panel fills in the settings below.
import contextlib, ipaddress, json, os, random, socket, ssl, sys, threading, time, urllib.error, urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

HOST = __HOST__
PATH = __PATH__
REPORT = __REPORT__
SIZE = __SIZE__
PER_RANGE = __PER_RANGE__
RANGES = __RANGES__
FRAG_IPS = __IPS__
TIMEOUT = 5
WORKERS = 24
FRAG_SAMPLE = 5
FRAG_TRIES = 2
# (packets, length, delay): Xray's two ways to cut the ClientHello. "tlshello" sends it as
# many small TLS records; "1-1" sends its one record as many small TCP segments.
FRAGMENTS = [(p, n, d) for p in ("tlshello", "1-1")
             for n, d in [("1-5", "1-2")] + [(n, d) for n in ("10-20", "20-50", "50-100", "100-200")
                                              for d in ("1-2", "5-10", "10-20")]]


def pick(cidr, n):
    net = ipaddress.ip_network(cidr)
    size = net.num_addresses - 2
    return {str(net.network_address + 1 + random.randrange(size)) for _ in range(min(n, size))}


def span(text):
    a, _, b = text.partition("-")
    return int(a), int(b or a)


def send_fragmented(raw, data, frag):
    packets, length, delay = frag
    lo, hi = span(length)
    dlo, dhi = span(delay)
    if len(data) < 5 or data[0] != 0x16:
        return raw.sendall(data)
    if packets != "tlshello":
        i = 0
        while i < len(data):
            part = data[i:i + random.randint(lo, hi)]
            i += len(part)
            raw.sendall(part)
            if i < len(data) and dhi:
                time.sleep(random.randint(dlo, dhi) / 1000)
        return
    n = int.from_bytes(data[3:5], "big")
    head, hello, rest = data[:3], data[5:5 + n], data[5 + n:]
    i = 0
    while i < len(hello):
        part = hello[i:i + random.randint(lo, hi)]
        i += len(part)
        raw.sendall(head + len(part).to_bytes(2, "big") + part)
        if i < len(hello) and dhi:
            time.sleep(random.randint(dlo, dhi) / 1000)
    if rest:
        raw.sendall(rest)


class FragTLS:
    """TLS through ssl.MemoryBIO so the ClientHello can be sent in pieces."""

    def __init__(self, raw, ctx, frag):
        self.raw, self.inc, self.out = raw, ssl.MemoryBIO(), ssl.MemoryBIO()
        self.obj = ctx.wrap_bio(self.inc, self.out, server_hostname=HOST)
        raw.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
        first = True
        while True:
            try:
                self.obj.do_handshake()
                break
            except ssl.SSLWantReadError:
                data = self.out.read()
                if first and data:
                    send_fragmented(raw, data, frag)
                    first = False
                elif data:
                    raw.sendall(data)
                chunk = raw.recv(16384)
                if not chunk:
                    raise ConnectionResetError("closed during TLS")
                self.inc.write(chunk)
        self._flush()

    def _flush(self):
        data = self.out.read()
        if data:
            self.raw.sendall(data)

    def settimeout(self, t):
        self.raw.settimeout(t)

    def sendall(self, data):
        self.obj.write(data)
        self._flush()

    def recv(self, n):
        while True:
            try:
                return self.obj.read(n)
            except ssl.SSLWantReadError:
                chunk = self.raw.recv(16384)
                if not chunk:
                    return b""
                self.inc.write(chunk)
            except ssl.SSLZeroReturnError:
                return b""


def probe(ip, frag=None):
    """TLS to ip with the CDN domain as SNI, then an upload of SIZE bytes; any HTTP answer
    means the data got through. Returns {ip, ms, colo} or None."""
    ctx = ssl.create_default_context()
    ctx.set_alpn_protocols(["http/1.1"])
    req = ("POST %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: Mozilla/5.0\r\nContent-Type: application/octet-stream\r\n"
           "Content-Length: %d\r\nConnection: close\r\n\r\n" % (PATH, HOST, SIZE)).encode() + os.urandom(SIZE)
    t0 = time.perf_counter()
    try:
        with socket.create_connection((ip, 443), timeout=TIMEOUT) as raw:
            with (contextlib.nullcontext(FragTLS(raw, ctx, frag)) if frag
                  else ctx.wrap_socket(raw, server_hostname=HOST)) as s:
                s.settimeout(TIMEOUT)
                s.sendall(req)
                head = s.recv(4096)
        ms = round((time.perf_counter() - t0) * 1000)
        if not head.startswith(b"HTTP/"):
            return None
        # Cloudflare's own 1xxx error (1034: this edge IP may not serve the domain)
        if head.split(None, 2)[1:2] == [b"403"] and b"\r\n\r\nerror code: 1" in head:
            return None
        colo = ""
        for line in head.split(b"\r\n"):
            if line.lower().startswith(b"cf-ray:"):
                colo = line.split(b"-")[-1].strip().decode(errors="replace")[:8]
        return {"ip": ip, "ms": ms, "colo": colo}
    except Exception:
        return None


def run(jobs, label):
    """jobs = [(ip, frag)]; prints progress, returns the results in job order."""
    out = [None] * len(jobs)
    done, lock = 0, threading.Lock()
    with ThreadPoolExecutor(WORKERS) as pool:
        futures = {pool.submit(probe, ip, frag): i for i, (ip, frag) in enumerate(jobs)}
        for fut in as_completed(futures):
            out[futures[fut]] = fut.result()
            with lock:
                done += 1
                sys.stdout.write("\r  %s: %d/%d " % (label, done, len(jobs)))
                sys.stdout.flush()
    print()
    return out


def main():
    print("3x-ui clean IP test via %s (%d-byte upload)" % (HOST, SIZE))
    print("Keep the VPN OFF: the test is for the internet this device is on.\n")

    ips = sorted({ip for cidr in RANGES for ip in pick(cidr, PER_RANGE)})
    random.shuffle(ips)
    found = sorted([r for r in run([(ip, None) for ip in ips], "IPs") if r], key=lambda r: r["ms"])
    print("  working: %d of %d" % (len(found), len(ips)))
    for r in found[:10]:
        print("    %-16s %5d ms  %s" % (r["ip"], r["ms"], r["colo"]))

    # Fragment: IPs that answer at all (the page's quick test, then this test's), else random.
    sample = list(dict.fromkeys(FRAG_IPS + [r["ip"] for r in found]))[:FRAG_SAMPLE]
    if len(sample) < FRAG_SAMPLE:
        sample += random.sample([ip for ip in ips if ip not in sample], FRAG_SAMPLE - len(sample))
    configs = [None] + FRAGMENTS
    jobs = [(ip, cfg) for _ in range(FRAG_TRIES) for cfg in configs for ip in sample]
    print("\nFragment test: %d settings and none, on %d IPs, %d times each" % (len(FRAGMENTS), len(sample), FRAG_TRIES))
    results = run(jobs, "fragment")
    rows = {cfg: {"ok": 0, "n": 0, "ms": 0} for cfg in configs}
    for (ip, cfg), r in zip(jobs, results):
        rows[cfg]["n"] += 1
        if r:
            rows[cfg]["ok"] += 1
            rows[cfg]["ms"] += r["ms"]
    fragments = []
    for cfg, row in rows.items():
        packets, length, delay = cfg or ("", "", "")
        fragments.append({"packets": packets, "length": length, "delay": delay, "ok": row["ok"], "n": row["n"],
                          "ms": round(row["ms"] / row["ok"]) if row["ok"] else 0})
    fragments.sort(key=lambda f: (-f["ok"], f["ms"] or 10 ** 9))
    for f in fragments[:6]:
        name = "%s %s %s" % (f["packets"], f["length"], f["delay"]) if f["packets"] else "no fragment"
        print("    %-26s %2d/%-2d %5d ms" % (name, f["ok"], f["n"], f["ms"]))

    body = json.dumps({"ips": found[:200], "tested": len(ips), "size": SIZE, "fragments": fragments}).encode()
    req = urllib.request.Request(REPORT, data=body, method="POST",
                                 headers={"Content-Type": "application/json", "User-Agent": "3x-ui-clean-ip"})
    for ctx in (ssl.create_default_context(), ssl._create_unverified_context()):
        try:
            with urllib.request.urlopen(req, timeout=30, context=ctx) as r:
                out = json.loads(r.read() or b"{}")
            print("\nSent to the panel (ISP: %s). Open the Clean IP scanner page to save." % (out.get("isp") or "?"))
            return
        except urllib.error.URLError as e:
            if isinstance(e.reason, ssl.SSLCertVerificationError):
                continue  # a panel with a self-signed certificate
            print("\nCould not send the result to the panel (%s)." % e)
            return
        except Exception as e:
            print("\nCould not send the result to the panel (%s)." % e)
            return


if __name__ == "__main__":
    main()
