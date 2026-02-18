# 09. Xray Protocol Reference for 3x-ui

## Scope

This document captures protocol-level understanding relevant to 3x-ui operations and implementation decisions.

Important:
- It is an operator-focused reference, not full wire-spec replacement for each protocol.
- Always validate against current upstream Xray docs before major production migrations.

## Protocols supported in current model/constants

From `database/model/model.go` protocol constants:
- `vmess`
- `vless`
- `trojan`
- `shadowsocks`
- `http`
- `mixed`
- `tunnel`
- `wireguard`

## Concept model

In Xray/3x-ui, configuration is split into:
1. Protocol/auth layer (vless/vmess/trojan/ss/etc)
2. Transport/stream layer (`tcp`, `ws`, `grpc`, etc)
3. Security layer (`none`, `tls`, `reality`)
4. Routing/DNS policy layer

Most operator mistakes happen when these layers are mixed conceptually.

## VLESS

Characteristics:
- Modern lightweight protocol (common choice with Reality).
- Client identity via UUID (`id`) and optional `flow` in specific TLS/XTLS contexts.

Typical production pairings:
- `vless + tcp + reality`
- `vless + ws + tls`
- `vless + grpc + tls`

Operational notes:
- Treat UUID/flow/security migration as coordinated change.
- Reality key material/shortIds are break-sensitive.

## VMESS

Characteristics:
- Legacy/common protocol with UUID identity and security parameters.

Operational notes:
- Can still work well, but many modern deployments prefer VLESS.
- Migration VMESS->VLESS should be staged as full client profile replacement.

## Trojan

Characteristics:
- Password-based identity model.
- Often used with TLS-like camouflage patterns.

3x-ui key behavior:
- Client key for updates/deletes is password for trojan path in service logic.

## Shadowsocks

Characteristics:
- Cipher/password model.
- Includes classic and 2022-style cipher handling paths in Xray integration.

3x-ui key behavior:
- Email plays tracking role in panel logic even for SS clients.
- For SS, client-key handling differs from UUID-based protocols.

## WireGuard / Tunnel / HTTP / Mixed

These exist in model support and can appear in inbound definitions depending on build/version/UI exposure.

Operational recommendation:
- Use these only when your use-case explicitly requires them.
- Keep your mainstream user path on one well-tested protocol family.

## Reality

Reality is a security/camouflage mode (commonly with VLESS TCP).

Key operator concerns:
- Public key/private key, shortIds, destination/serverName consistency.
- Any mismatch causes hard connect failures.
- Keep a fallback inbound during rotations.

## XHTTP / HTTP-style obfuscation context

In this session context, “xhttp-like/client-similar behavior” referred to HTTP header camouflage over TCP-style transport.

Implemented test pattern:
- `vless-tcp-http-18080-test`
- Request header host and path set for camouflage testing.

Guidance:
- Keep obfuscation experiments in clearly marked test inbounds.
- Do not blend experimental obfuscation into main production inbound without staged rollout.

## Routing and DNS interplay (critical)

Protocol success can still fail at policy layer due to:
- DNS resolver split policies
- Domain/IP route overrides
- Direct/proxy exceptions

Always validate:
1. Inbound reaches server
2. Auth/security valid
3. DNS route correct
4. Outbound path available

## Practical compatibility checklist before creating a new inbound type

1. Confirm protocol support in current panel build.
2. Confirm client app supports same protocol+transport+security combo.
3. Create isolated test inbound first.
4. Add one test client and verify handshake/traffic.
5. Roll out with explicit naming and fallback path.

## Naming convention reminder

Use descriptive remarks to prevent operator error:
- `<protocol>-<transport>-<security>-<port>-<role>`

Examples used in session:
- `vless-reality-tcp-443-main`
- `vless-reality-tcp-8443-alt`
- `vless-tcp-http-18080-test`

## Where to cross-check in codebase

- Protocol constants: `database/model/model.go`
- Inbound/client update logic: `web/service/inbound.go`
- Xray add/remove user and inbound operations: `xray/api.go`
- Xray config assembly: `web/service/xray.go`
