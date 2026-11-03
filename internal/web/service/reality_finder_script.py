#!/usr/bin/env python3
# 3x-ui REALITY target finder. Run it with the VPN OFF, on the internet your users have.
# Standard library only (Python 3.8+). The panel fills in the settings below.
SERVER = __SERVER__
PORT = __PORT__
SCOPE = __SCOPE__
KNOWN = __KNOWN__
REPORT = __REPORT__
LIMIT = 4096
WORKERS = 16
TOP = 100

import sys
import concurrent.futures as cf
import ipaddress
import json
import os
import queue
import re
import socket
import ssl
import struct
import threading
import time
import urllib.request


UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"
X25519 = 0x001D
TLS13 = 0x0304
# ServerHello.random of a HelloRetryRequest (RFC 8446 4.1.3)
HRR_RANDOM = bytes.fromhex("CF21AD74E59A6111BE1D8C021E65B891C2A211167ABB8C5E079E09E2C8A8339C")
GROUPS = {0x001D: "x25519", 0x0017: "secp256r1", 0x0018: "secp384r1", 0x0019: "secp521r1",
          0x001E: "x448", 0x11EC: "X25519MLKEM768", 0x6399: "X25519Kyber768"}
ALERTS = {40: "handshake_failure", 70: "protocol_version", 71: "insufficient_security",
          80: "internal_error", 112: "unrecognized_name", 47: "illegal_parameter"}
SCOPES = ("24", "22", "20", "prefix", "asn")
MAX_IPS = 20000



def ssl_context(verify=True):
    """TLS client context. Where the system has no CA store Python can read (the Android
    app), fall back to certifi's bundle when it is installed."""
    ctx = ssl.create_default_context()
    if not verify:
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE
        return ctx
    paths = ssl.get_default_verify_paths()
    if not ctx.get_ca_certs() and not (paths.cafile or paths.capath):
        try:
            import certifi
            ctx.load_verify_locations(certifi.where())
        except Exception:
            pass
    return ctx


# ---------- raw TLS 1.3 ClientHello: offers only X25519 ----------

def _ext(kind, data):
    return struct.pack("!HH", kind, len(data)) + data


def client_hello(sni):
    """A TLS 1.3 ClientHello record that offers X25519 as its only key-exchange group.
    Python's ssl module can't pick the group, so the hello is built by hand: a server
    without X25519 answers with an alert or a HelloRetryRequest."""
    exts = b""
    if sni:
        name = sni.encode("idna")
        entry = b"\x00" + struct.pack("!H", len(name)) + name
        exts += _ext(0, struct.pack("!H", len(entry)) + entry)
    exts += _ext(10, struct.pack("!HH", 2, X25519))                                  # supported_groups
    exts += _ext(11, b"\x01\x00")                                                    # ec_point_formats
    sigs = (0x0403, 0x0804, 0x0401, 0x0503, 0x0805, 0x0501, 0x0806, 0x0601, 0x0807)
    exts += _ext(13, struct.pack("!H", 2 * len(sigs)) + b"".join(struct.pack("!H", s) for s in sigs))
    alpn = b"\x02h2\x08http/1.1"
    exts += _ext(16, struct.pack("!H", len(alpn)) + alpn)                            # ALPN
    exts += _ext(43, b"\x02\x03\x04")                                                # supported_versions: 1.3
    exts += _ext(45, b"\x01\x01")                                                    # psk_dhe_ke
    share = struct.pack("!HH", X25519, 32) + os.urandom(32)
    exts += _ext(51, struct.pack("!H", len(share)) + share)                          # key_share
    ciphers = b"\x13\x01\x13\x02\x13\x03"
    body = (b"\x03\x03" + os.urandom(32) + b"\x20" + os.urandom(32)
            + struct.pack("!H", len(ciphers)) + ciphers + b"\x01\x00"
            + struct.pack("!H", len(exts)) + exts)
    hs = b"\x01" + struct.pack("!I", len(body))[1:] + body
    return b"\x16\x03\x01" + struct.pack("!H", len(hs)) + hs


def parse_server_hello(data):
    """What the server chose: {'version', 'group', 'cipher', 'hrr'} or {'alert': name}.
    data = the bytes read after the ClientHello. Raises ValueError when it isn't TLS."""
    hs = b""
    pos = 0
    while True:
        if len(data) < pos + 5:
            raise ValueError("short")
        kind, length = data[pos], struct.unpack("!H", data[pos + 3:pos + 5])[0]
        rec = data[pos + 5:pos + 5 + length]
        if len(rec) < length:
            raise ValueError("short")
        pos += 5 + length
        if kind == 21 and len(rec) >= 2:
            return {"alert": ALERTS.get(rec[1], f"alert_{rec[1]}")}
        if kind != 22:
            raise ValueError("not_tls")
        hs += rec
        if len(hs) >= 4 and len(hs) >= 4 + int.from_bytes(hs[1:4], "big"):
            break
    if hs[0] != 2:
        raise ValueError("not_server_hello")
    msg = hs[4:4 + int.from_bytes(hs[1:4], "big")]
    random_ = msg[2:34]
    p = 34
    p += 1 + msg[p]                                   # session id
    cipher = struct.unpack("!H", msg[p:p + 2])[0]
    p += 3                                            # cipher + compression
    out = {"version": struct.unpack("!H", msg[0:2])[0], "group": None, "cipher": cipher,
           "hrr": random_ == HRR_RANDOM}
    if len(msg) < p + 2:
        return out
    end = p + 2 + struct.unpack("!H", msg[p:p + 2])[0]
    p += 2
    while p + 4 <= end:
        kind, length = struct.unpack("!HH", msg[p:p + 4])
        val = msg[p + 4:p + 4 + length]
        if kind == 43 and length == 2:
            out["version"] = struct.unpack("!H", val)[0]
        elif kind == 51 and length >= 2:
            out["group"] = struct.unpack("!H", val[:2])[0]
        p += 4 + length
    return out


def _read_hello(sock, limit=16384):
    """Read records until the first handshake message (or an alert) is complete."""
    data = b""
    while len(data) < limit:
        chunk = sock.recv(4096)
        if not chunk:
            break
        data += chunk
        try:
            return parse_server_hello(data)
        except ValueError as e:
            if str(e) != "short":
                raise
    return parse_server_hello(data)


def x25519_check(ip, sni, port=443, timeout=4):
    """(ok, detail): does the server finish a TLS 1.3 hello when only X25519 is offered?"""
    try:
        with socket.create_connection((ip, port), timeout=timeout) as s:
            s.settimeout(timeout)
            s.sendall(client_hello(sni))
            r = _read_hello(s)
    except (OSError, ValueError) as e:
        return False, f"hello: {e}"[:80]
    if "alert" in r:
        return False, r["alert"]
    if r["hrr"]:
        return False, "wants " + GROUPS.get(r["group"], hex(r["group"] or 0))
    if r["version"] != TLS13:
        return False, "no TLS 1.3"
    if r["group"] != X25519:
        return False, GROUPS.get(r["group"], hex(r["group"] or 0))
    return True, "x25519"


# ---------- normal TLS: certificate, version, ALPN ----------

def tls_info(ip, sni, port=443, timeout=4, verify=True):
    """Handshake as a browser would: {'version', 'alpn', 'cert', 'ms'} or {'error'}.
    sni=None: no SNI, any certificate accepted when verify=False (to read the names)."""
    ctx = ssl_context(verify)
    if verify and not sni:
        ctx.check_hostname = False
    try:
        ctx.minimum_version = ssl.TLSVersion.TLSv1_2
    except (AttributeError, ValueError):
        pass
    ctx.set_alpn_protocols(["h2", "http/1.1"])
    t0 = time.perf_counter()
    try:
        with socket.create_connection((ip, port), timeout=timeout) as raw:
            with ctx.wrap_socket(raw, server_hostname=sni or None) as s:
                ms = (time.perf_counter() - t0) * 1000
                return {"version": s.version(), "alpn": s.selected_alpn_protocol() or "",
                        "cert": s.getpeercert() or {}, "der": s.getpeercert(binary_form=True) or b"",
                        "ms": ms}
    except ssl.SSLCertVerificationError as e:
        return {"error": "bad_cert", "detail": str(getattr(e, "verify_message", "") or e)[:80]}
    except ssl.SSLError as e:
        return {"error": "tls", "detail": (e.reason or str(e))[:80]}
    except socket.timeout:
        # from inside a filtered network a timeout is often the dest being blocked here
        return {"error": "timeout", "detail": "no answer (or blocked on this network)"}
    except OSError as e:
        return {"error": "conn", "detail": str(e)[:80]}


_DNS_RE = re.compile(r"^(?=.{4,253}$)([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$")


def cert_names(cert):
    """DNS names in a certificate (from ssl getpeercert()): exact names first, then wildcards."""
    names = [v.lower() for k, v in cert.get("subjectAltName", ()) if k == "DNS"]
    for rdn in cert.get("subject", ()):
        for k, v in rdn:
            if k == "commonName":
                names.append(v.lower())
    names = list(dict.fromkeys(names))
    return [n for n in names if "*" not in n] + [n for n in names if n.startswith("*.")]


def der_names(der):
    """DNS names from a DER certificate without verifying it (CN + SAN dNSName).
    Small ASN.1 walk: enough for reading names, not a parser for anything else."""
    names = []
    for m in re.finditer(rb"\x06\x03\x55\x04\x03(?:\x0c|\x13|\x16)([\x00-\x7f])", der):
        n = m.group(1)[0]
        names.append(der[m.end():m.end() + n].decode("ascii", "replace"))
    i = der.find(b"\x06\x03\x55\x1d\x11")             # subjectAltName OID
    if i >= 0:
        j = i + 5
        if der[j:j + 1] == b"\x01":                   # critical flag
            j += 3
        if der[j:j + 1] == b"\x04":                   # OCTET STRING
            j += 1
            ln = der[j]
            j += 1 + (ln - 0x80 if ln & 0x80 else 0)
            if der[j:j + 1] == b"\x30":               # SEQUENCE of GeneralName
                ln = der[j + 1]
                k = j + 2
                if ln & 0x80:
                    nb = ln - 0x80
                    ln = int.from_bytes(der[k:k + nb], "big")
                    k += nb
                end = k + ln
                while k < end and k < len(der):
                    tag, ln = der[k], der[k + 1]
                    k += 2
                    if ln & 0x80:
                        nb = ln - 0x80
                        ln = int.from_bytes(der[k:k + nb], "big")
                        k += nb
                    if tag == 0x82:
                        names.append(der[k:k + ln].decode("ascii", "replace"))
                    k += ln
    names = [n.lower() for n in names if _DNS_RE.match(n.lower().lstrip("*."))]
    names = list(dict.fromkeys(names))
    return [n for n in names if "*" not in n] + [n for n in names if n.startswith("*.")]


def candidates(names, limit=3):
    """SNIs worth trying from a certificate's names: exact names, then www.<wildcard base>."""
    out = []
    for n in names:
        n = ("www." + n[2:]) if n.startswith("*.") else n
        if _DNS_RE.match(n) and n not in out:
            out.append(n)
    return out[:limit]


# ---------- network facts: CDN, ASN, prefixes ----------

_CDN_NETS = None


def cdn_of_ip(ip):
    """'cloudflare' / 'fastly' / ... when the IP is in a CDN's published ranges, else ''."""
    global _CDN_NETS
    if _CDN_NETS is None:
        _CDN_NETS = [(k, ipaddress.ip_network(r)) for k, p in PROVIDERS.items()
                     for _, _, rs in p["groups"] for r in rs]
    addr = ipaddress.ip_address(ip)
    for k, net in _CDN_NETS:
        if addr.version == net.version and addr in net:
            return k
    return ""


def resolve(name, timeout=4):
    """IPv4 + IPv6 addresses of a name (empty on failure)."""
    out = []
    box = []

    def go():
        try:
            box.extend(socket.getaddrinfo(name, 443, proto=socket.IPPROTO_TCP))
        except OSError:
            pass
    t = threading.Thread(target=go, daemon=True)
    t.start()
    t.join(timeout)
    for fam, _, _, _, addr in list(box):
        if addr[0] not in out:
            out.append(addr[0])
    return out


def _get_json(url, timeout=8):
    req = urllib.request.Request(url, headers={"User-Agent": "Reality-Scanner"})
    with urllib.request.urlopen(req, timeout=timeout, context=ssl_context()) as r:
        return json.loads(r.read(4_000_000))


_ASN_CACHE = {}
_ASN_LOCK = threading.Lock()


def ip_network_info(ip, timeout=8):
    """{'asn': int|None, 'prefix': 'a.b.c.0/x'|None, 'holder': str} from RIPEstat (cached)."""
    with _ASN_LOCK:
        if ip in _ASN_CACHE:
            return _ASN_CACHE[ip]
    out = {"asn": None, "prefix": None, "holder": ""}
    try:
        d = _get_json(f"https://stat.ripe.net/data/network-info/data.json?resource={ip}", timeout)["data"]
        if d.get("asns"):
            out["asn"] = int(d["asns"][0])
        out["prefix"] = d.get("prefix") or None
        if out["asn"]:
            h = _get_json(f"https://stat.ripe.net/data/as-overview/data.json?resource=AS{out['asn']}", timeout)
            out["holder"] = str(h["data"].get("holder") or "")
    except Exception:
        pass
    with _ASN_LOCK:
        _ASN_CACHE[ip] = out
    return out


def asn_prefixes(asn, timeout=15):
    """IPv4 prefixes the AS announces (RIPEstat), or []."""
    try:
        d = _get_json(f"https://stat.ripe.net/data/announced-prefixes/data.json?resource=AS{asn}", timeout)
        return [p["prefix"] for p in d["data"]["prefixes"] if ":" not in p["prefix"]]
    except Exception:
        return []


NET_SOURCES = (
    ("https://speed.cloudflare.com/meta",
     lambda d: (d.get("asn"), d.get("asOrganization"), d.get("country"), d.get("clientIp"))),
    ("https://ipwho.is/",
     lambda d: ((d.get("connection") or {}).get("asn"), (d.get("connection") or {}).get("org"),
                d.get("country_code"), d.get("ip"))),
)
CLOUDFLARE_ASNS = {13335, 14789, 209242, 395747}
HOSTING_RE = re.compile(
    r"hosting|hetzner|ovh|digitalocean|linode|akamai|vultr|choopa|contabo|leaseweb|scaleway|"
    r"amazon|aws|google|microsoft|azure|oracle|alibaba|tencent|m247|ionos|hostinger|datacamp|"
    r"data ?cent|colocation|server|vps", re.I)


def detect_network(timeout=6):
    """Which network (ISP) this device is on: {'asn', 'org', 'country', 'ip'} or None."""
    for url, pick in NET_SOURCES:
        try:
            req = urllib.request.Request(url, headers={"User-Agent": UA, "Referer": "https://speed.cloudflare.com/"})
            with urllib.request.urlopen(req, timeout=timeout, context=ssl_context()) as r:
                asn, org, country, ip = pick(json.loads(r.read(65536)))
            if asn:
                return {"asn": int(asn), "org": str(org or ""), "country": str(country or ""), "ip": str(ip or "")}
        except Exception:
            continue
    return None


def network_warning(network):
    """'vpn' (WARP / a VPN is on: the filter test is not for an ISP), 'hosting' (running on
    a server: the filter test means nothing there) or ''."""
    if not network:
        return ""
    if network.get("asn") in CLOUDFLARE_ASNS:
        return "vpn"
    return "hosting" if HOSTING_RE.search(network.get("org") or "") else ""


# ---------- targets ----------

def server_address(server):
    """The server's IPv4 address from an IP or a host name (ValueError when none)."""
    s = str(server or "").strip().lower()
    for p in ("https://", "http://", "vless://"):
        if s.startswith(p):
            s = s[len(p):]
    s = s.split("/")[0].split("@")[-1]
    if s.startswith("[") or s.count(":") > 1:
        raise ValueError("ipv6")
    s = s.split(":")[0]
    try:
        addr = ipaddress.ip_address(s)
    except ValueError:
        ips = [a for a in resolve(s) if ":" not in a]
        if not ips:
            raise ValueError("no_dns")
        addr = ipaddress.ip_address(ips[0])
    if addr.version != 4:
        raise ValueError("ipv6")
    if not addr.is_global:
        raise ValueError("not_public")
    return str(addr)


def scope_networks(server_ip, scope, info=None):
    """Networks to scan for a scope: '24', '22', '20' around the server, its announced
    'prefix', or every IPv4 prefix of its 'asn'."""
    if scope in ("24", "22", "20"):
        return [ipaddress.ip_network(f"{server_ip}/{scope}", strict=False)]
    info = info or ip_network_info(server_ip)
    if scope == "prefix" and info.get("prefix") and ":" not in info["prefix"]:
        return [ipaddress.ip_network(info["prefix"], strict=False)]
    if scope == "asn" and info.get("asn"):
        nets = [ipaddress.ip_network(p, strict=False) for p in asn_prefixes(info["asn"])]
        if nets:
            return nets
    return [ipaddress.ip_network(f"{server_ip}/24", strict=False)]


def ips_near(server_ip, nets, limit=MAX_IPS):
    """Host addresses of nets, nearest to the server first, at most limit. The server's
    own network is walked outwards from the server; other prefixes evenly after it."""
    srv = int(ipaddress.ip_address(server_ip))
    home = [n for n in nets if ipaddress.ip_address(server_ip) in n]
    rest = [n for n in nets if n not in home]
    out, seen = [], {srv}
    for net in home:
        lo, hi = int(net.network_address) + 1, int(net.broadcast_address) - 1
        for d in range(1, max(srv - lo, hi - srv) + 1):
            for v in (srv - d, srv + d):
                if lo <= v <= hi and v not in seen:
                    seen.add(v)
                    out.append(v)
                    if len(out) >= limit:
                        return [str(ipaddress.ip_address(x)) for x in out]
    # other prefixes: walk them all in parallel, a slice of each per round
    rest.sort(key=lambda n: abs(int(n.network_address) - srv))
    iters = [iter(range(int(n.network_address) + 1, int(n.broadcast_address))) for n in rest]
    while iters and len(out) < limit:
        alive = []
        for it in iters:
            for _ in range(16):
                v = next(it, None)
                if v is None:
                    break
                if v not in seen:
                    seen.add(v)
                    out.append(v)
            else:
                alive.append(it)
            if len(out) >= limit:
                break
        iters = alive
    return [str(ipaddress.ip_address(x)) for x in out[:limit]]


# ---------- the filter test (through the user's own server) ----------

def is_tls_alert(error):
    # OpenSSL names every standard alert *_ALERT_* or TLSV1_* (TLSV1_UNRECOGNIZED_NAME)
    reason = getattr(error, "reason", None) or ""
    return isinstance(error, ssl.SSLError) and ("ALERT" in reason or reason.startswith("TLSV1_"))


def sni_through(server_ip, port, sni, timeout=6, tries=2):
    """Does a TLS handshake with this SNI to the server get through this network?
    A REALITY server passes unknown names on to its dest, so the dest answers unless the
    ISP cuts the name. A TLS alert is such an answer, even though it refuses the name."""
    ctx = ssl_context(verify=False)
    for i in range(tries):
        try:
            with socket.create_connection((server_ip, port), timeout=timeout) as raw:
                raw.settimeout(timeout)
                with ctx.wrap_socket(raw, server_hostname=sni):
                    return True
        except (OSError, ValueError) as e:
            if is_tls_alert(e):
                return True
            if i + 1 < tries:
                time.sleep(0.5)
    return False


def sni_router_check(results, ctx):
    """True when the filter test can't be trusted: (almost) every target came back filtered while
    the inbound's own dest name (ctx known_sni) gets through. An SNI router in front of
    REALITY (haproxy, nginx stream, a DNS proxy) only passes the names it knows; a
    whitelisting ISP looks the same. Those rows are then marked untested and rescored."""
    known = ctx.get("known_sni")
    tested = [r for r in results if r.get("ok") and r.get("filtered") is not None]
    cut = [r for r in tested if r["filtered"]]
    # the few names that pass are usually the router's other known names
    if not known or not ctx.get("server_port") or len(cut) < 3 or len(tested) - len(cut) > max(1, len(tested) // 10):
        return False
    if not sni_through(ctx["server_ip"], ctx["server_port"], known, ctx.get("timeout", 4) + 2):
        return False
    for r in cut:
        r["filtered"] = None
        r["score"] = score(r)
    return True


def tcp_open(ip, port, timeout=5):
    try:
        with socket.create_connection((ip, port), timeout=timeout):
            return True
    except OSError:
        return False


# ---------- one target ----------

def same24(a, b):
    return a.split(".")[:3] == b.split(".")[:3]


def score(r):
    """0-100. Only targets with TLS 1.3 + X25519 + h2 + a valid certificate get here."""
    s = 0
    if r.get("filtered") is False:
        s += 30
    elif r.get("filtered") is None:
        s += 12                                       # not tested
    if r.get("same24"):
        s += 25
    elif r.get("same_asn"):
        s += 15
    if not r.get("cdn"):
        s += 15
    if r.get("dns_match"):
        s += 15
    ms = r.get("ms") or 9999
    s += 15 if ms < 150 else 11 if ms < 300 else 7 if ms < 600 else 3 if ms < 1200 else 0
    if r.get("filtered"):
        s = min(s, 20)
    return s


def check_name(ip, sni, port=443, timeout=4):
    """TLS 1.3 + h2 + valid certificate for sni, then the X25519-only hello.
    {'ok': True, 'ms', 'names'} or {'ok': False, 'why'}."""
    t = tls_info(ip, sni, port, timeout)
    if "error" in t:
        return {"ok": False, "why": t["error"] + (": " + t["detail"] if t.get("detail") else "")}
    if t["version"] != "TLSv1.3":
        return {"ok": False, "why": f"no TLS 1.3 ({t['version']})"}
    if t["alpn"] != "h2":
        return {"ok": False, "why": "no h2"}
    ok, detail = x25519_check(ip, sni, port, timeout)
    if not ok:
        return {"ok": False, "why": detail}
    return {"ok": True, "ms": t["ms"], "names": cert_names(t["cert"]),
            "issuer": _issuer(t["cert"]), "expires": t["cert"].get("notAfter", "")}


def _issuer(cert):
    d = {}
    for rdn in cert.get("issuer", ()):
        for k, v in rdn:
            d[k] = v
    return d.get("organizationName") or d.get("commonName") or ""


def scan_ip(ip, ctx):
    """One neighbour IP: read the names from its certificate, check them. A result dict or None."""
    port = ctx.get("dest_port", 443)
    first = tls_info(ip, None, port, ctx["timeout"], verify=False)
    if "error" in first:
        return None
    names = der_names(first["der"])
    for sni in candidates(names):
        r = check_name(ip, sni, port, ctx["timeout"])
        if r["ok"]:
            return finish(ip, sni, r, ctx)
    return None


def scan_domain(name, ctx):
    """One domain from a list: resolve it, check it. A result dict, or a failed one with 'why'."""
    ips = [a for a in resolve(name) if ":" not in a]
    if not ips:
        return {"ok": False, "sni": name, "why": "no DNS"}
    r = check_name(ips[0], name, ctx.get("dest_port", 443), ctx["timeout"])
    if not r["ok"]:
        return {"ok": False, "sni": name, "ip": ips[0], "why": r["why"]}
    return finish(ips[0], name, r, ctx, ips)


def finish(ip, sni, r, ctx, dns=None):
    """Add the network facts, the filter test and the score to a passing target."""
    dns = dns if dns is not None else resolve(sni)
    v4 = [a for a in dns if ":" not in a]
    out = {"ok": True, "ip": ip, "sni": sni, "ms": round(r["ms"]), "issuer": r["issuer"],
           "expires": r["expires"], "names": [n for n in r["names"] if "*" not in n][:8],
           "dns": v4[:4], "dns_match": ip in v4,
           "cdn": next((c for c in (cdn_of_ip(a) for a in [ip] + v4) if c), ""),
           "same24": same24(ip, ctx["server_ip"]), "same_asn": ctx.get("scope_same_asn", False),
           "filtered": None}
    if not out["same24"] and not out["same_asn"] and ctx.get("server_asn"):
        out["same_asn"] = ip_network_info(ip).get("asn") == ctx["server_asn"]
    if ctx.get("server_port") and ctx.get("server_reachable", True):
        out["filtered"] = not sni_through(ctx["server_ip"], ctx["server_port"], sni)
    out["score"] = score(out)
    return out


def reality_settings(r):
    """The part of an inbound's realitySettings this target fills in."""
    return {"dest": f"{r['sni']}:443", "serverNames": [r["sni"]]}


# ---------- the scanner ----------

class Scanner:
    """Scans targets on worker threads.
    on_result(result) for every passing target (and, in domain mode, every failed one),
    on_progress(done, total), on_done(); .stop is an Event."""

    def __init__(self, targets, ctx, workers=16, mode="ip",
                 on_result=None, on_progress=None, on_done=None):
        self.targets, self.ctx, self.workers, self.mode = list(targets), ctx, workers, mode
        self.on_result = on_result or (lambda r: None)
        self.on_progress = on_progress or (lambda d, t: None)
        self.on_done = on_done or (lambda: None)
        self.stop = threading.Event()
        self.done = 0
        self.lock = threading.Lock()
        self.seen = set()

    def start(self):
        threading.Thread(target=self.run, daemon=True).start()

    def run(self):
        q = queue.Queue()
        for t in self.targets:
            q.put(t)
        fn = scan_ip if self.mode == "ip" else scan_domain

        def worker():
            while not self.stop.is_set():
                try:
                    t = q.get_nowait()
                except queue.Empty:
                    return
                try:
                    r = fn(t, self.ctx)
                except Exception as e:                # never let one target kill a worker
                    r = {"ok": False, "sni": t, "why": f"error: {e}"[:80]} if self.mode != "ip" else None
                with self.lock:
                    self.done += 1
                    done = self.done
                    if r and r.get("ok"):
                        if r["sni"] in self.seen:     # one row per name
                            r = None
                        else:
                            self.seen.add(r["sni"])
                if r:
                    self.on_result(r)
                self.on_progress(done, len(self.targets))
        try:
            with cf.ThreadPoolExecutor(self.workers) as ex:
                for _ in range(self.workers):
                    ex.submit(worker)
        finally:
            self.on_done()


def prepare(server, server_port=443, scope="24", domains=None, timeout=4, limit=MAX_IPS, log=print, known_sni=""):
    """Everything a scan needs: (targets, ctx, mode). Raises ValueError with a code.
    known_sni: the inbound's current dest name, the control for sni_router_check()."""
    server_ip = server_address(server)
    ctx = {"server_ip": server_ip, "server_port": int(server_port or 0), "timeout": float(timeout),
           "known_sni": known_sni if known_sni and _DNS_RE.match(known_sni) else ""}
    info = ip_network_info(server_ip)
    ctx["server_asn"], ctx["server_holder"], ctx["server_prefix"] = info["asn"], info["holder"], info["prefix"]
    if ctx["server_port"]:
        ctx["server_reachable"] = tcp_open(server_ip, ctx["server_port"])
        if not ctx["server_reachable"]:
            log(f"! {server_ip}:{ctx['server_port']} does not answer from here: filter test skipped")
    if domains:
        names = []
        for d in domains:
            d = d.strip().lower().split("#")[0].strip()
            for p in ("https://", "http://"):
                if d.startswith(p):
                    d = d[len(p):]
            d = d.split("/")[0].split(":")[0]
            if d and _DNS_RE.match(d) and d not in names:
                names.append(d)
        if not names:
            raise ValueError("no_targets")
        return names[:limit], ctx, "domain"
    if scope not in SCOPES:
        raise ValueError("bad_scope")
    nets = scope_networks(server_ip, scope, info)
    ctx["scope_same_asn"] = scope in ("prefix", "asn") or bool(info["asn"])
    if scope in ("24", "22", "20") and info["asn"]:
        # a /22 or /20 can cross into another AS: only claim it for the server's own prefix
        pfx = ipaddress.ip_network(info["prefix"], strict=False) if info["prefix"] and ":" not in info["prefix"] else None
        ctx["scope_same_asn"] = bool(pfx and nets[0].subnet_of(pfx)) if pfx else False
    ctx["scope"] = scope
    ctx["networks"] = [str(n) for n in nets[:50]]
    return ips_near(server_ip, nets, limit), ctx, "ip"


# published CDN ranges (the scanner's default groups)
PROVIDERS = {
    'cloudflare': {"groups": [
        ('official', True, """
173.245.48.0/20 103.21.244.0/22 103.22.200.0/22 103.31.4.0/22 141.101.64.0/18
108.162.192.0/18 190.93.240.0/20 188.114.96.0/20 197.234.240.0/22 198.41.128.0/17
162.158.0.0/15 104.16.0.0/13 104.24.0.0/14 172.64.0.0/13 131.0.72.0/22
""".split()),
        ('popular', True, """
185.16.110.0/24 45.85.118.0/23 185.193.28.0/22 185.148.104.0/22 170.114.45.0/24 8.6.112.0/24
8.6.144.0/23 8.9.231.0/24 5.10.214.0/23 141.193.213.0/24 23.227.38.0/23 66.235.200.0/24
154.83.2.0/24 194.152.44.0/24 5.226.179.0/24
""".split()),
    ]},
    'fastly': {"groups": [
        ('official', True, """
23.235.32.0/20 43.249.72.0/22 103.244.50.0/24 103.245.222.0/23 103.245.224.0/24
104.156.80.0/20 140.248.64.0/18 140.248.128.0/17 146.75.0.0/17 151.101.0.0/16 157.52.64.0/18
167.82.0.0/17 167.82.128.0/20 167.82.160.0/20 167.82.224.0/20 172.111.64.0/18 185.31.16.0/22
199.27.72.0/21 199.232.0.0/16
""".split()),
    ]},
    'gcore': {"groups": [
        ('as199524', True, """
2.56.220.0/22 5.8.24.0/22 5.8.32.0/22 5.8.40.0/23 5.8.68.0/24 5.8.70.0/23 5.8.92.0/22
5.45.184.0/22 5.101.216.0/23 5.101.219.0/24 5.101.220.0/24 5.101.222.0/23 5.181.25.0/24
5.181.27.0/24 5.181.76.0/22 5.188.0.0/23 5.188.4.0/23 5.188.6.0/24 5.188.32.0/23
5.188.34.0/24 5.188.36.0/24 5.188.70.0/23 5.188.92.0/22 5.188.108.0/22 5.188.120.0/24
5.188.122.0/23 5.188.124.0/22 5.188.132.0/22 5.188.148.0/24 5.188.151.0/24 5.188.168.0/23
5.188.188.0/24 5.188.190.0/23 5.188.213.0/24 5.188.214.0/23 5.188.225.0/24 5.188.226.0/23
5.188.228.0/24 5.188.230.0/23 5.188.238.0/23 5.189.200.0/24 5.189.202.0/23 5.189.204.0/24
5.189.206.0/23 5.189.220.0/23 5.189.222.0/24 5.252.24.0/22 31.13.213.0/24 31.184.206.0/23
31.184.244.0/23 31.184.246.0/24 31.223.184.0/24 37.9.32.0/22 37.61.224.0/24 37.61.229.0/24
37.61.231.0/24 37.202.196.0/24 45.10.58.0/23 45.65.9.0/24 45.65.10.0/23 45.67.208.0/22
45.80.208.0/21 45.82.30.0/23 45.82.100.0/22 45.82.160.0/22 45.92.70.0/23 45.135.228.0/22
45.146.203.0/24 45.147.160.0/23 50.20.200.0/24 62.112.216.0/23 62.112.218.0/24
62.112.221.0/24 62.112.222.0/23 66.248.194.0/23 66.248.196.0/24 74.112.76.0/23
78.111.99.0/24 78.111.100.0/24 78.111.102.0/24 78.111.104.0/23 78.111.107.0/24
78.111.108.0/23 78.111.110.0/24 78.138.52.0/24 79.133.108.0/22 79.133.120.0/22
79.133.124.0/23 79.133.126.0/24 80.93.209.0/24 80.93.210.0/23 80.93.214.0/23 80.93.217.0/24
80.93.218.0/23 80.93.222.0/23 80.240.96.0/23 80.240.101.0/24 80.240.104.0/23 80.240.112.0/22
80.240.124.0/24 81.28.12.0/22 82.97.203.0/24 82.97.205.0/24 82.97.206.0/24 82.117.224.0/23
82.117.226.0/24 82.117.228.0/24 82.158.112.0/21 82.158.120.0/24 82.158.123.0/24
82.158.124.0/23 83.229.4.0/24 83.229.6.0/24 83.229.15.0/24 83.229.32.0/24 85.90.192.0/23
85.90.194.0/24 85.90.204.0/22 85.90.212.0/24 85.90.215.0/24 85.90.216.0/23 85.204.246.0/24
85.234.65.0/24 85.234.68.0/22 85.234.72.0/22 85.234.76.0/23 85.234.80.0/23 85.234.83.0/24
85.234.84.0/22 85.234.88.0/23 85.234.92.0/24 85.234.94.0/23 86.105.169.0/24 86.106.169.0/24
87.86.215.0/24 87.120.106.0/24 87.120.167.0/24 87.120.202.0/24 87.120.204.0/24
87.120.237.0/24 87.120.252.0/24 87.121.33.0/24 87.121.39.0/24 87.121.53.0/24 87.121.55.0/24
87.121.99.0/24 89.35.162.0/24 89.36.229.0/24 89.38.100.0/24 89.39.70.0/24 89.39.82.0/24
89.39.161.0/24 89.39.191.0/24 89.40.75.0/24 89.41.172.0/23 89.42.178.0/24 89.43.104.0/21
89.44.192.0/21 89.223.88.0/23 89.223.91.0/24 89.223.92.0/22 91.188.250.0/24 91.199.82.0/24
91.199.84.0/24 91.199.87.0/24 91.206.163.0/24 91.239.65.0/24 91.243.81.0/24 91.243.85.0/24
91.243.87.0/24 92.38.132.0/22 92.38.141.0/24 92.38.142.0/24 92.38.144.0/23 92.38.146.0/24
92.38.148.0/22 92.38.154.0/23 92.38.157.0/24 92.38.158.0/23 92.38.160.0/20 92.38.176.0/21
92.38.184.0/22 92.223.0.0/22 92.223.7.0/24 92.223.10.0/23 92.223.12.0/23 92.223.15.0/24
92.223.16.0/20 92.223.39.0/24 92.223.40.0/24 92.223.42.0/24 92.223.44.0/22 92.223.50.0/23
92.223.52.0/23 92.223.54.0/24 92.223.56.0/22 92.223.61.0/24 92.223.62.0/23 92.223.66.0/24
92.223.68.0/22 92.223.73.0/24 92.223.74.0/23 92.223.76.0/22 92.223.81.0/24 92.223.82.0/23
92.223.84.0/23 92.223.86.0/24 92.223.88.0/23 92.223.90.0/24 92.223.92.0/22 92.223.96.0/22
92.223.100.0/23 92.223.102.0/24 92.223.104.0/23 92.223.107.0/24 92.223.112.0/23
92.223.116.0/22 92.223.120.0/23 92.223.124.0/22 92.243.82.0/24 92.243.88.0/24 92.243.93.0/24
93.113.36.0/23 93.114.56.0/24 93.114.68.0/24 93.114.98.0/24 93.115.240.0/23 93.119.170.0/24
93.123.11.0/24 93.123.17.0/24 93.123.38.0/24 93.123.102.0/24 93.174.164.0/23 94.156.45.0/24
94.156.93.0/24 94.156.99.0/24 94.176.183.0/24 94.177.131.0/24 95.85.64.0/20 95.85.89.0/24
95.85.90.0/23 95.85.92.0/22 96.62.73.0/24 103.22.192.0/22 103.30.211.0/24 103.215.216.0/24
103.215.218.0/23 109.61.16.0/22 109.61.32.0/20 109.61.120.0/24 109.61.122.0/23
109.61.124.0/23 109.61.127.0/24 109.107.159.0/24 139.28.4.0/22 144.34.86.0/24 146.19.94.0/24
146.185.214.0/24 146.185.217.0/24 146.185.219.0/24 146.185.236.0/23 146.185.248.0/22
150.107.125.0/24 150.107.126.0/24 151.246.170.0/24 158.62.206.0/23 162.33.20.0/24
162.120.2.0/23 162.213.60.0/23 162.213.63.0/24 162.216.230.0/23 165.217.130.0/23
165.217.132.0/22 165.217.143.0/24 166.0.21.0/24 166.0.29.0/24 169.155.122.0/23
169.155.124.0/23 176.126.122.0/24 178.107.17.0/24 180.149.36.0/22 180.149.44.0/22
184.170.200.0/22 185.12.212.0/24 185.12.240.0/24 185.12.242.0/23 185.14.44.0/23
185.14.47.0/24 185.67.124.0/22 185.101.136.0/24 185.101.138.0/23 185.105.0.0/22
185.109.31.0/24 185.145.113.0/24 185.160.60.0/23 185.163.0.0/22 185.183.128.0/22
185.188.144.0/22 185.202.92.0/22 185.244.208.0/22 185.249.132.0/22 188.208.32.0/23
188.241.71.0/24 188.241.250.0/24 192.108.254.0/23 192.135.112.0/23 192.135.116.0/24
192.135.118.0/23 193.36.180.0/23 193.57.88.0/23 193.138.111.0/24 193.142.214.0/24
193.169.238.0/23 193.169.250.0/23 193.239.179.0/24 193.242.96.0/24 194.5.182.0/24
194.99.78.0/23 194.99.80.0/23 195.234.62.0/23 195.234.82.0/23 199.115.72.0/22
199.115.77.0/24 199.115.78.0/23 199.180.206.0/23 202.78.160.0/23 202.78.163.0/24
202.78.164.0/24 202.78.167.0/24 202.78.168.0/21 205.179.220.0/22 205.179.240.0/22
209.102.248.0/23 209.102.251.0/24 209.102.252.0/22 212.6.46.0/24 213.156.136.0/21
213.156.144.0/20 216.201.72.0/23 216.201.78.0/23 217.28.137.0/24 217.195.193.0/24
217.195.194.0/24 217.195.200.0/23 217.195.205.0/24
""".split()),
    ]},
    'cloudfront': {"groups": [
        ('official', True, """
3.160.0.0/13 3.168.0.0/14 3.172.0.0/17 3.173.0.0/16 3.174.0.0/15 13.32.0.0/15 13.35.0.0/16
13.224.0.0/14 13.249.0.0/16 15.158.0.0/16 18.64.0.0/14 18.68.0.0/16 18.154.0.0/15
18.160.0.0/15 18.164.0.0/15 18.172.0.0/15 18.238.0.0/15 18.244.0.0/15 23.91.0.0/19
23.228.212.0/23 23.228.214.0/24 23.228.220.0/22 23.228.244.0/24 23.228.246.0/23
23.228.248.0/22 23.234.192.0/18 24.110.32.0/19 24.110.128.0/17 36.103.232.0/25
36.103.232.128/26 51.74.192.0/18 52.46.0.0/18 52.82.128.0/19 52.84.0.0/15 52.124.128.0/17
52.222.128.0/17 54.182.0.0/16 54.192.0.0/16 54.230.0.0/17 54.230.128.0/18 54.230.200.0/21
54.230.208.0/20 54.230.224.0/19 54.239.128.0/18 54.239.192.0/19 54.240.128.0/18
58.254.138.0/25 58.254.138.128/26 64.252.64.0/18 64.252.128.0/18 65.8.0.0/16 65.9.0.0/17
65.9.128.0/18 70.132.0.0/18 71.152.0.0/17 99.84.0.0/16 99.86.0.0/16 108.138.0.0/15
108.156.0.0/14 111.13.171.128/25 111.13.185.32/27 111.13.185.64/27 116.129.226.0/25
116.129.226.128/26 118.193.97.64/26 118.193.97.128/25 119.147.182.0/25 119.147.182.128/26
120.52.12.64/26 120.52.22.96/27 120.52.39.128/27 120.52.153.192/26 120.232.236.0/25
120.232.236.128/26 120.253.240.192/26 120.253.241.160/27 120.253.245.128/26
120.253.245.192/27 130.176.0.0/17 130.176.128.0/18 130.176.192.0/19 130.176.224.0/20
143.204.0.0/16 144.220.0.0/16 180.163.57.0/25 180.163.57.128/26 204.246.164.0/22
204.246.168.0/21 204.246.176.0/20 205.251.202.0/23 205.251.204.0/22 205.251.208.0/20
205.251.249.0/24 205.251.250.0/23 205.251.252.0/23 205.251.254.0/24 216.137.32.0/19
""".split()),
    ]},
}


def ask(text):
    if sys.stdin and sys.stdin.isatty():
        try:
            input(text)
        except EOFError:
            pass


def row(r):
    return {"sni": r["sni"], "ip": r["ip"], "score": r["score"], "ms": r["ms"], "filtered": r["filtered"],
            "same24": r["same24"], "sameAsn": bool(r["same_asn"]), "cdn": r["cdn"], "dnsMatch": r["dns_match"],
            "issuer": r["issuer"], "expires": r["expires"]}


def send(report):
    data = json.dumps(report).encode()
    for attempt in range(3):
        try:
            req = urllib.request.Request(REPORT, data=data, headers={"Content-Type": "application/json", "User-Agent": UA})
            with urllib.request.urlopen(req, timeout=30, context=ssl_context()) as r:
                ans = json.loads(r.read(65536) or b"{}")
            if ans.get("success"):
                return True
            print("The panel refused the report:", ans.get("msg") or "?")
            return False
        except Exception as e:
            print(f"Could not reach the panel: {e}")
            if attempt < 2:
                ask("If the panel only opens with the VPN, turn the VPN ON now and press Enter to send again... ")
    path = os.path.abspath("reality-report.json")
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(report, fh, indent=1)
    print("Saved the results to", path)
    return False


def run():
    network = detect_network() or {}
    warn = network_warning(network)
    print(f"Your network: AS{network.get('asn', '?')} {network.get('org', '?')} ({network.get('country', '?')})")
    if warn:
        print("! This looks like a VPN or a server, not your ISP: the filter test would be wrong.")
        ask("Turn the VPN off and press Enter (or Ctrl+C to stop)... ")
        network = detect_network() or network
        warn = network_warning(network)
    try:
        targets, ctx, mode = prepare(SERVER, PORT, SCOPE, None, 4, LIMIT, known_sni=KNOWN)
    except ValueError as e:
        raise SystemExit(f"error: {e}")
    print(f"Server {ctx['server_ip']}  AS{ctx['server_asn'] or '?'} {ctx['server_holder']}  prefix {ctx['server_prefix'] or '?'}")
    print(f"Testing {len(targets)} IPs next to it with {WORKERS} workers...")
    found = []
    lock = threading.Lock()

    def add(r):
        if r.get("ok"):
            with lock:
                found.append(r)
            print(f"\r  + {r['sni']:<40} {r['ip']:<16} {r['ms']:>5} ms  score {r['score']}" + " " * 10)

    def prog(done, total):
        print(f"\r  {done}/{total}, found {len(found)} ", end="", flush=True)

    sc = Scanner(targets, ctx, WORKERS, mode, add, prog)
    try:
        sc.run()
    except KeyboardInterrupt:
        sc.stop.set()
        print("\nStopped: sending what was found so far.")
    router = sni_router_check(found, ctx)
    found.sort(key=lambda r: (-r["score"], r["ms"]))
    print(f"\n\nFound {len(found)} targets.")
    report = {"network": {"asn": int(network.get("asn") or 0), "org": network.get("org", ""),
                          "country": network.get("country", "")},
              "warning": warn, "router": router, "tested": sc.done,
              "server": {"ip": ctx["server_ip"], "asn": int(ctx["server_asn"] or 0),
                         "holder": ctx["server_holder"] or "", "prefix": ctx["server_prefix"] or ""},
              "port": ctx["server_port"], "filterTested": bool(ctx["server_port"] and ctx.get("server_reachable", True)),
              "targets": [row(r) for r in found[:TOP]]}
    if send(report):
        print("Sent. Open the REALITY target scanner in your panel to pick one.")


if __name__ == "__main__":
    run()
