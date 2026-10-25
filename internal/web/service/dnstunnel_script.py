#!/usr/bin/env python3
# 3x-ui DNS tunnel resolver test. Run it with the VPN OFF, on the internet (ISP) to test.
# Standard library only (Python 3.8+). The panel fills in the settings below.
import json, os, random, socket, ssl, string, struct, subprocess, sys, time, urllib.error, urllib.request
from concurrent.futures import ThreadPoolExecutor

ZONE = __ZONE__
RESOLVERS = __RESOLVERS__
REPORT = __REPORT__
TIMEOUT = 3
TRIES = 2
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


def build_query(name):
    tid = random.getrandbits(16)
    header = struct.pack(">HHHHHH", tid, 0x0100, 1, 0, 0, 1)
    qname = b"".join(bytes([len(label)]) + label.encode() for label in name.split(".")) + b"\x00"
    opt = b"\x00" + struct.pack(">HHIH", 41, 1232, 0, 0)  # EDNS0, 1232-byte UDP
    return tid, header + qname + struct.pack(">HH", 16, 1) + opt  # TXT


def probe(ip):
    """A random name under the tunnel zone can only be answered by our server: NOERROR or
    NXDOMAIN means this resolver reaches it; SERVFAIL / no answer means it does not."""
    family = socket.AF_INET6 if ":" in ip else socket.AF_INET
    status, best = "timeout", 0
    for _ in range(TRIES):
        label = "".join(random.choice(string.ascii_lowercase + string.digits) for _ in range(12))
        tid, packet = build_query(label + "." + ZONE)
        t0 = time.perf_counter()
        try:
            with socket.socket(family, socket.SOCK_DGRAM) as s:
                s.settimeout(TIMEOUT)
                s.sendto(packet, (ip, 53))
                while True:
                    data, _ = s.recvfrom(4096)
                    if len(data) >= 12 and struct.unpack(">H", data[:2])[0] == tid:
                        break
            ms = round((time.perf_counter() - t0) * 1000)
            rcode = struct.unpack(">H", data[2:4])[0] & 0xF
            status = {0: "ok", 3: "ok", 2: "servfail", 5: "refused"}.get(rcode, "error")
            best = ms
            if status == "ok":
                break
        except socket.timeout:
            best = TIMEOUT * 1000
        except OSError:
            status, best = "error", 0
    return {"ip": ip, "status": status, "ms": best, "name": KNOWN.get(ip, "")}


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
    results.sort(key=lambda r: (r["status"] != "ok", r["ms"]))
    for r in results:
        print("  %-3s %-40s %5d ms  %s" % ("OK" if r["status"] == "ok" else "--", r["ip"], r["ms"],
                                            r["name"] or r["status"]))
    ok = [r["ip"] for r in results if r["status"] == "ok"]
    print("\nWorking resolvers (%d):\n%s" % (len(ok), "\n".join(ok) or "none"))

    body = json.dumps({"results": results}).encode()
    req = urllib.request.Request(REPORT, data=body, method="POST",
                                 headers={"Content-Type": "application/json", "User-Agent": "3x-ui-dns-tunnel"})
    for ctx in (ssl.create_default_context(), ssl._create_unverified_context()):
        try:
            with urllib.request.urlopen(req, timeout=30, context=ctx) as r:
                out = json.loads(r.read() or b"{}")
            print("\nSent to the panel (ISP: %s). Open the DNS tunnel page to see it." % (out.get("isp") or "?"))
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
