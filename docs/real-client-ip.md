# Capturing the Real Client IP

When an Xray inbound sits behind an intermediary — a CDN like Cloudflare, an L4 tunnel/relay,
or another panel — the IP that Xray sees is the **intermediary's** address, not the visitor's.
That intermediary IP is what shows up in the panel's online/IP view and what the per-client
**IP limit** counts against, which makes both useless behind a proxy.

Xray-core can recover the real visitor IP. 3x-ui exposes the two mechanisms in the inbound form
and feeds the recovered IP into the same pipeline that drives IP-limit enforcement, the online
list, and multi-node sync — so once it is set, everything downstream just works.

## Where to set it

Open an inbound → **Transport / Stream Settings** → enable **Sockopt** → use the
**Real client IP** preset selector:

| Preset | What it does | Use for |
|---|---|---|
| **Off / direct** | Clears both fields. | Inbound reachable directly by clients. |
| **Cloudflare CDN** | Sets `sockopt.trustedXForwardedFor = ["CF-Connecting-IP"]`. | WebSocket / HTTPUpgrade / XHTTP behind Cloudflare's CDN (orange cloud). |
| **L4 relay / Spectrum (PROXY)** | Sets `acceptProxyProtocol = true`. | An L4 tunnel/relay in front, or Cloudflare **Spectrum**. |

The raw `Proxy Protocol` switch and `Trusted X-Forwarded-For` list stay visible below the preset
selector for manual / advanced tuning — the presets just fill them in for you.

## Scenario 1 — Cloudflare CDN

Cloudflare's CDN (the orange cloud) forwards the visitor's IP in the `CF-Connecting-IP` request
header. Xray reads it when the transport is **WebSocket**, **HTTPUpgrade**, or **XHTTP** and
the header name is listed in `sockopt.trustedXForwardedFor`.

```json
"streamSettings": {
  "network": "ws",
  "sockopt": { "trustedXForwardedFor": ["CF-Connecting-IP"] }
}
```

Pick the **Cloudflare CDN** preset. You can add `X-Real-IP`, `True-Client-IP`, or `X-Client-IP`
to the list if a different upstream uses those.

> This is **not** the same as Cloudflare Spectrum. The free/CDN tier forwards HTTP headers — use
> this scenario. Spectrum (a TCP/L4 product) can send the PROXY protocol — use Scenario 2.

## Scenario 2 — L4 tunnel / relay or Cloudflare Spectrum (PROXY protocol)

For a TCP-level front (HAProxy, gost, nginx `stream`, an Xray dokodemo-door relay, or Cloudflare
Spectrum), the real IP is carried in the **PROXY protocol** header. Enable
`acceptProxyProtocol` and make sure the **upstream emits PROXY protocol** — otherwise the
connection will fail.

```json
"streamSettings": {
  "network": "tcp",
  "sockopt": { "acceptProxyProtocol": true }
}
```

Pick the **L4 relay / Spectrum (PROXY)** preset. Works on TCP/RAW, WebSocket, HTTPUpgrade, gRPC
and XHTTP; **not** on mKCP. The front must be configured to send the header, e.g.:

- **HAProxy**: `server backend 127.0.0.1:443 send-proxy` (or `send-proxy-v2`).
- **nginx** (`stream {}` block): `proxy_protocol on;` on the `server`, and on the upstream side
  `proxy_protocol on;` in the `server` that connects to Xray.

## Transport support matrix

| Mechanism | TCP/RAW | mKCP | WebSocket | gRPC | HTTPUpgrade | XHTTP |
|---|:--:|:--:|:--:|:--:|:--:|:--:|
| `trustedXForwardedFor` (header) | – | – | ✅ | – | ✅ | ✅ |
| `acceptProxyProtocol` (PROXY)   | ✅ | – | ✅ | ✅ | ✅ | ✅ |

The form shows a warning when you select a preset that the current transport cannot honor.

> **Use one, not both.** `acceptProxyProtocol` and `trustedXForwardedFor` are independent — the
> first reads the real IP from the L4 PROXY header, the second from an HTTP request header. On
> WebSocket / HTTPUpgrade / XHTTP, xray applies the HTTP header *last*, so a stale
> `trustedXForwardedFor` would override (and defeat) a PROXY-protocol setup. The presets are
> mutually exclusive and clear the other field for you; only mix them by hand if you know your
> upstream chain needs it.

## Multi-node

No extra configuration is needed. The inbound's `streamSettings` (including these sockopt
fields) is pushed to child nodes verbatim, so the node's Xray records the real IP, and the
parent panel pulls each node's per-client IPs roughly every 10 seconds. The real visitor IP
shows up on the parent automatically.

## Security note

Both `acceptProxyProtocol` and `trustedXForwardedFor` are **server-side only** — they are
stripped from subscription output, so they never reach clients. Only enable
`trustedXForwardedFor` when the inbound is genuinely behind a trusted proxy that sets the
header; otherwise a client could spoof the header and forge its own source IP.

## Verifying

1. Set the preset and save the inbound.
2. Inspect the generated Xray config and confirm `streamSettings.sockopt` carries the expected
   field (`trustedXForwardedFor` or `acceptProxyProtocol`).
3. Connect through the intermediary, then open the client's IPs / online view in the panel — it
   should show the real visitor IP rather than the CDN/relay address.
