#!/usr/bin/env python3
# 3x-ui DNS tunnel resolver test. Run it with the VPN OFF, on the internet (ISP) to test.
# Standard library only (Python 3.8+). The panel fills in the settings below.
import json, os, random, socket, ssl, string, struct, subprocess, sys, time, urllib.request
from concurrent.futures import ThreadPoolExecutor

ZONE = __ZONE__
RESOLVERS = __RESOLVERS__
REPORT = __REPORT__
TIMEOUT = 3
TRIES = 2
PORT = 53
# A DNS name is at most 253 characters. StormDNS packs its upload into the query name, so a
# resolver that drops long names forces a small upload MTU on every resolver in the list.
NAME_MAX = 253
# Public resolvers people in Iran use; the panel's list (above) and this device's own come first.
KNOWN = {
    "178.22.122.100": "Shecan", "185.51.200.2": "Shecan", "78.157.42.100": "Electro", "78.157.42.101": "Electro",
    "10.202.10.10": "Radar", "10.202.10.11": "Radar", "10.202.10.202": "403", "10.202.10.102": "403",
    "185.55.226.26": "Begzar", "185.55.225.25": "Begzar", "5.202.100.100": "Pishgaman", "5.202.100.101": "Pishgaman",
    "85.15.1.14": "Shatel", "85.15.1.15": "Shatel", "217.218.155.155": "TCI", "217.218.127.127": "TCI",
    "8.8.8.8": "Google", "8.8.4.4": "Google", "1.1.1.1": "Cloudflare", "1.0.0.1": "Cloudflare",
    "9.9.9.9": "Quad9", "208.67.222.222": "OpenDNS", "94.140.14.14": "AdGuard",
}


def system_resolvers():
    """This device's own DNS servers (the ISP's, on a phone or a home router)."""
    found = []
    for path in ("/etc/resolv.conf", os.path.join(os.environ.get("PREFIX", "/data/data/com.termux/files/usr"), "etc/resolv.conf")):
        try:
            for line in open(path):
                parts = line.split()
                if len(parts) >= 2 and parts[0] == "nameserver":
                    found.append(parts[1])
        except OSError:
            pass
    for prop in ("net.dns1", "net.dns2"):
        try:
            out = subprocess.run(["getprop", prop], capture_output=True, text=True, timeout=3).stdout.strip()
            if out:
                found.append(out)
        except Exception:
            pass
    if sys.platform == "win32":
        try:
            out = subprocess.run(["powershell", "-NoProfile", "-Command",
                                  "(Get-DnsClientServerAddress -AddressFamily IPv4).ServerAddresses"],
                                 capture_output=True, text=True, timeout=10).stdout
            found += [l.strip() for l in out.splitlines() if l.strip()]
        except Exception:
            pass
    bad = ("127.", "0.", "198.18.", "198.19.")  # loopback, or a VPN's fake DNS
    return [ip for ip in dict.fromkeys(found) if not ip.startswith(bad)]


def build_query(name, qtype=16):
    tid = random.getrandbits(16)
    header = struct.pack(">HHHHHH", tid, 0x0100, 1, 0, 0, 1)
    qname = b"".join(bytes([len(label)]) + label.encode() for label in name.split(".")) + b"\x00"
    opt = b"\x00" + struct.pack(">HHIH", 41, 1232, 0, 0)  # EDNS0, 1232-byte UDP
    return tid, header + qname + struct.pack(">HH", qtype, 1) + opt  # TXT by default


def rand_label(n):
    return "".join(random.choice(string.ascii_lowercase + string.digits) for _ in range(n))


def zone_name(length):
    """A random name under ZONE exactly length characters long, in labels of at most 63."""
    left, labels = length - len(ZONE) - 1, []
    while left > 0:
        n = min(63, left)
        if left - n == 1:  # a lone dot would be left over: make this label one shorter
            n -= 1
        labels.append(rand_label(n))
        left -= n + 1
    return ".".join(labels + [ZONE])


def ask(ip, name, qtype=16):
    """One query; returns (rcode, answer count, ms), or None when no answer comes."""
    family = socket.AF_INET6 if ":" in ip else socket.AF_INET
    tid, packet = build_query(name, qtype)
    t0 = time.perf_counter()
    with socket.socket(family, socket.SOCK_DGRAM) as s:
        s.settimeout(TIMEOUT)
        s.sendto(packet, (ip, PORT))
        try:
            while True:
                data, _ = s.recvfrom(4096)
                if len(data) >= 12 and struct.unpack(">H", data[:2])[0] == tid:
                    break
        except socket.timeout:
            return None
    rcode = struct.unpack(">H", data[2:4])[0] & 0xF
    return rcode, struct.unpack(">H", data[6:8])[0], round((time.perf_counter() - t0) * 1000)


def reaches(ip, name):
    """NOERROR or NXDOMAIN for a random name under the zone can only come from our server."""
    for _ in range(TRIES):
        got = ask(ip, name)
        if got:
            return got[0] in (0, 3)
    return False


def max_name(ip):
    """The longest query name (characters) this resolver still carries to our server."""
    lo, hi = len(ZONE) + 13, NAME_MAX
    if reaches(ip, zone_name(hi)):
        return hi
    while lo < hi - 1:
        mid = (lo + hi) // 2
        if reaches(ip, zone_name(mid)):
            lo = mid
        else:
            hi = mid
    return lo


def probe(ip):
    """status: ok (reaches our server), liar (also makes answers up, so its "ok" proves
    nothing), servfail / refused / timeout / error (does not reach it)."""
    status, best = "timeout", 0
    try:
        for _ in range(TRIES):
            got = ask(ip, zone_name(len(ZONE) + 13))
            if got is None:
                best = TIMEOUT * 1000
                continue
            status = {0: "ok", 3: "ok", 2: "servfail", 5: "refused"}.get(got[0], "error")
            best = got[2]
            if status == "ok":
                break
        if status == "ok":
            fake = ask(ip, rand_label(12) + "." + rand_label(6) + ".invalid", 1)
            if fake and fake[0] == 0 and fake[1] > 0:
                status = "liar"
    except OSError:
        status, best = "error", 0
    row = {"ip": ip, "status": status, "ms": best, "name": KNOWN.get(ip, "")}
    if status == "ok":
        row["maxName"] = max_name(ip)
    return row


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


def shared_list(results):
    """StormDNS runs every resolver at the smallest MTU among them, so resolvers that only
    carry short names are left out while others work. -> (ips, low, left_out)"""
    ok = [r for r in results if r["status"] == "ok"]
    full = [r["ip"] for r in ok if r["maxName"] >= NAME_MAX]
    if full:
        return full, False, len(ok) - len(full)
    return [r["ip"] for r in ok], bool(ok), 0


def main():
    mine = system_resolvers()
    targets = list(dict.fromkeys(mine + RESOLVERS + list(KNOWN)))
    print("3x-ui DNS tunnel resolver test for %s: %d resolvers" % (ZONE, len(targets)))
    print("Keep the VPN OFF: the test is for the internet this device is on.\n")
    with ThreadPoolExecutor(16) as pool:
        results = list(pool.map(probe, targets))
    for r in results:
        if r["ip"] in mine and not r["name"]:
            r["name"] = "this device"
    results.sort(key=lambda r: (r["status"] != "ok", r.get("maxName", 0) < NAME_MAX, r["ms"]))
    for r in results:
        low = r["status"] == "ok" and r["maxName"] < NAME_MAX
        print("  %-3s %-40s %5d ms  %s%s" % ("OK" if r["status"] == "ok" else "--", r["ip"], r["ms"],
                                              r["name"] or r["status"],
                                              "  (low MTU: names up to %d characters)" % r["maxName"] if low else ""))
    ips, low, left_out = shared_list(results)
    if low:
        print("\nWorking only with small packets (%d):\n%s" % (len(ips), "\n".join(ips)))
        print("Set MIN_UPLOAD_MTU = 40 and MIN_DOWNLOAD_MTU = 200 in the StormDNS client to use them.")
    else:
        print("\nWorking resolvers (%d):\n%s" % (len(ips), "\n".join(ips) or "none"))
        if left_out:
            print("\n%d more work only with small packets: left out, as StormDNS would run every"
                  " resolver at their small MTU." % left_out)

    body = json.dumps({"results": results}).encode()
    req = urllib.request.Request(REPORT, data=body, method="POST",
                                 headers={"Content-Type": "application/json", "User-Agent": "3x-ui-dns-tunnel"})
    try:
        with urllib.request.urlopen(req, timeout=30, context=ssl_context()) as r:
            out = json.loads(r.read() or b"{}")
        print("\nSent to the panel (ISP: %s). Open the DNS tunnel page to see it." % (out.get("isp") or "?"))
    except Exception as e:
        print("\nCould not send the result to the panel (%s)." % e)


if __name__ == "__main__":
    main()
