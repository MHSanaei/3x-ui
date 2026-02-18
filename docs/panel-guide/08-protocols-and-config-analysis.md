# 08. Protocols and Config Analysis (Session Transfer)

## Scope

This file captures protocol and configuration understanding gathered during the session, including practical guidance for creating client-similar inbound behavior in 3x-ui.

## Xray protocol framing

Common protocols in this panel context:
- `vless`
- `vmess`
- `trojan`
- `shadowsocks`

Important distinction:
- Protocol defines identity/auth semantics.
- Transport/stream settings define wire behavior (tcp/ws/grpc/http-like headers/reality/tls/etc).

## Reality summary (operational)

Reality is used as a TLS-camouflage security layer in modern Xray setups (commonly with VLESS).

Typical panel representation:
- Protocol: `vless`
- Network: `tcp`
- Security marker: `Reality`

Operational notes:
- It is sensitive to keypair, shortIds, target/serverName settings.
- Changing Reality material on active inbounds can break all existing clients immediately.
- Use staged migration for key/shortId rotations.

## XHTTP / HTTP-like obfuscation context

In practical user terms (for this session), “similar config from client perspective” meant creating an inbound with:
- `vless` + `tcp`
- No TLS/Reality on that specific test inbound
- HTTP header camouflage (request path/host)

Implemented test-like inbound pattern in panel:
- Remark pattern after cleanup: `vless-tcp-http-18080-test`
- Port: `18080`
- HTTP obfuscation with `Host = speedtest.com`, `Path = /`

## Analysis of provided sample JSON configs

Two provided JSON samples were client-side local Xray configs, not panel server-side inbound objects.

High-level structure observed:
- Local inbounds:
  - SOCKS and HTTP on loopback + LAN IP (`10808`, `10809`)
  - Additional direct SOCKS inbound (`10820`) for bypass route
- Outbounds:
  - Main `vless` outbound with `tcp` + HTTP header disguise
  - `direct` (freedom)
  - `block` (blackhole)
- Routing:
  - Domain/IP direct lists
  - Special rules for DNS endpoints and direct inbound tag
- DNS:
  - DoH to Cloudflare and Google with domain-based resolver split

Interpretation:
- These configs are client aggregator profiles routing app traffic through a remote VLESS endpoint.
- To create “similar behavior” on your 3x-ui server, we focused on matching protocol/transport/obfuscation characteristics in inbound settings (not mirroring client local socks/http listener topology).

## Can similar inbounds be created in your instance?

Yes. Practical answer from this session:
- Similar client-facing server profile can be created in 3x-ui.
- Exact local client config object is not a server inbound; it must be translated to compatible inbound parameters.

## Safe conversion rules used

1. Preserve existing production inbounds (`443/8443 Reality`) to avoid breakage.
2. Add separate test inbound for obfuscated TCP behavior.
3. Use explicit remark naming to avoid accidental edits.
4. Verify saved settings from modal after creation.

## Naming standard adopted

- `vless-reality-tcp-443-main`
- `vless-reality-tcp-8443-alt`
- `vless-tcp-http-18080-test`

Format:
- `<protocol>-<security/transport>-<port>-<role>` (or close equivalent used here)

## Practical recommendations

- Keep Reality as production path when already stable for clients.
- Keep obfuscation test profiles isolated and clearly marked `-test`.
- Do not mix experimental configs into main inbound until tested and monitored.
