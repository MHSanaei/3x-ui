#!/usr/bin/env python3
# 3x-ui clean IP test. Run it with the VPN OFF, on the internet (ISP) you want IPs for.
# Standard library only (Python 3.8+). The panel fills in the settings below.
import contextlib, ipaddress, json, os, random, re, socket, ssl, sys, threading, time, urllib.parse, urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

HOST = __HOST__
PATH = __PATH__
REPORT = __REPORT__
PANEL_VIA = __VIA__
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


def targets():
    """The page's quick-scan hits first (so they get a datacenter too), then random edge IPs."""
    rest = sorted({ip for cidr in RANGES for ip in pick(cidr, PER_RANGE)} - set(FRAG_IPS))
    random.shuffle(rest)
    return list(dict.fromkeys(FRAG_IPS)) + rest


def colo_of(head):
    """Datacenter code from the reply headers of each CDN: Cloudflare CF-RAY (...-FRA), Fastly
    X-Served-By (the last cache-..-AMS is the edge nearest you), CloudFront X-Amz-Cf-Pop
    (IST50-P2), Gcore X-ID (lon1-up-gc30)."""
    h = {}
    for line in head.split(b"\r\n")[1:]:
        name, _, value = line.partition(b":")
        h[name.strip().lower()] = value.strip().decode(errors="replace")
    if b"cf-ray" in h:
        code = h[b"cf-ray"].rpartition("-")[2]
    elif b"x-served-by" in h:
        code = h[b"x-served-by"].split(",")[-1].strip().rpartition("-")[2]
    elif b"x-amz-cf-pop" in h:
        code = h[b"x-amz-cf-pop"][:3]
    else:
        m = re.match(r"([a-z]{3})\d*-", h.get(b"x-id", ""))
        code = m.group(1) if m else ""
    code = code.upper()
    return code if re.fullmatch(r"[A-Z]{3}", code) else ""


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


FAILS, FAILS_LOCK = {}, threading.Lock()
FAIL_HINTS = {
    "certificate": "The IPs did not show a certificate for %s: is it proxied (orange cloud) on this CDN?" % HOST,
    "timeout": "Everything timed out: this ISP may block %s by its name (SNI) - see the fragment test below." % HOST,
    "reset": "Connections were reset: this ISP's filter cuts %s (SNI) - see the fragment test below." % HOST,
    "closed-after-upload": "The upload was cut: this ISP drops uploads to %s; try a smaller upload." % HOST,
    "cloudflare-1xxx": "Cloudflare refused %s on these IPs (error 1xxx): they may not serve this domain." % HOST,
}


def failed(reason):
    with FAILS_LOCK:
        FAILS[reason] = FAILS.get(reason, 0) + 1


def failure_summary():
    return ", ".join("%s %d" % kv for kv in sorted(FAILS.items(), key=lambda kv: -kv[1]))


def ssl_context():
    """Phones (Termux, some Python builds) may have no CA store Python can read: use certifi's."""
    ctx = ssl.create_default_context()
    paths = ssl.get_default_verify_paths()
    if not ctx.get_ca_certs() and not (paths.cafile or paths.capath):
        try:
            import certifi
            ctx.load_verify_locations(certifi.where())
        except Exception:
            pass
    return ctx


def probe(ip, frag=None):
    """TLS to ip with the CDN domain as SNI, then an upload of SIZE bytes; any HTTP answer
    means the data got through. Returns {ip, ms, colo} or None (the reason goes to FAILS)."""
    ctx = ssl_context()
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
            failed("no-http-reply" if head else "closed-after-upload")
            return None
        # Cloudflare's own 1xxx error (1034: this edge IP may not serve the domain)
        if head.split(None, 2)[1:2] == [b"403"] and b"\r\n\r\nerror code: 1" in head:
            failed("cloudflare-1xxx")
            return None
        return {"ip": ip, "ms": ms, "colo": colo_of(head.split(b"\r\n\r\n")[0])}
    except ssl.SSLCertVerificationError:
        failed("certificate")
    except (socket.timeout, TimeoutError):
        failed("timeout")
    except ConnectionRefusedError:
        failed("refused")
    except ConnectionResetError:
        failed("reset")
    except ssl.SSLError:
        failed("tls-error")
    except Exception as e:
        failed(type(e).__name__)
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


def fragment_sample(found, ips):
    """Up to FRAG_SAMPLE IPs: the page's and this run's working ones first, then random targets."""
    sample = list(dict.fromkeys(FRAG_IPS + [r["ip"] for r in found]))[:FRAG_SAMPLE]
    rest = [ip for ip in ips if ip not in sample]
    return sample + random.sample(rest, min(len(rest), FRAG_SAMPLE - len(sample)))


def main():
    print("3x-ui clean IP test via %s (%d-byte upload)" % (HOST, SIZE))
    print("Keep the VPN OFF: the test is for the internet this device is on.\n")

    ips = targets()
    found = sorted([r for r in run([(ip, None) for ip in ips], "IPs") if r], key=lambda r: r["ms"])
    print("  working: %d of %d" % (len(found), len(ips)))
    for r in found[:10]:
        print("    %-16s %5d ms  %s" % (r["ip"], r["ms"], r["colo"]))
    if FAILS:
        print("  failed: " + failure_summary())
        top = max(FAILS, key=FAILS.get)
        if not found:
            print("  " + FAIL_HINTS.get(top, "Nothing got through: check the domain and that the VPN is off."))
    FAILS.clear()

    # Fragment: IPs that answer at all (the page's quick test, then this test's), else random.
    sample = fragment_sample(found, ips)
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
    if FAILS and not any(f["ok"] for f in fragments):
        print("  failed: " + failure_summary())

    body = json.dumps({"ips": found[:200], "tested": len(ips), "size": SIZE, "fragments": fragments}).encode()
    pin_panel()
    try:
        isp = send_report(body)
        print("\nSent to the panel (ISP: %s). Open the Clean IP scanner page to save." % isp)
    except Exception as e:
        print("\nCould not send the result to the panel (%s)." % e)


def pin_panel():
    """The command came through PANEL_VIA (curl --resolve): report through it too, as this
    ISP may not resolve or route the panel's domain."""
    if not PANEL_VIA:
        return
    host, resolve = urllib.parse.urlsplit(REPORT).hostname, socket.getaddrinfo
    socket.getaddrinfo = lambda h, *a, **k: resolve(PANEL_VIA if h == host else h, *a, **k)


def send_report(body):
    """POSTs the result; returns the ISP name the panel saw it from."""
    req = urllib.request.Request(REPORT, data=body, method="POST",
                                 headers={"Content-Type": "application/json", "User-Agent": "3x-ui-clean-ip"})
    return post_report(req, ssl_context())


def post_report(req, ctx):
    with urllib.request.urlopen(req, timeout=30, context=ctx) as r:
        return json.loads(r.read() or b"{}").get("isp") or "?"


if __name__ == "__main__":
    main()
