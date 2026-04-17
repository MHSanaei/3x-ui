# Xray-core fuzzing, stages 1-2

This package contains Go fuzz targets for Xray-core only. It does not fuzz x-ui, the web panel, Telegram bot, installer scripts, Docker wiring, or any external control plane.

## Targets

| Target | Surface | Main code paths |
| --- | --- | --- |
| `FuzzXrayCoreFullConfigBuild` | Full JSON config | `infra/conf/serial.DecodeJSONConfig` -> `infra/conf.Config.Build` -> `core.New` |
| `FuzzXrayCoreInboundVLESSConfigBuild` | Inbound and VLESS inbound fragments | `encoding/json` -> `infra/conf.InboundDetourConfig.Build` / `VLessInboundConfig.Build` -> `conf.Config.Build` -> `core.New` |
| `FuzzXrayCoreOutboundConfigBuild` | Outbound and VLESS outbound fragments | `encoding/json` -> `infra/conf.OutboundDetourConfig.Build` / `VLessOutboundConfig.Build` -> `conf.Config.Build` -> `core.New` |
| `FuzzXrayCoreStreamSettingsBuild` | Stream, transport, and security settings | `encoding/json` -> `infra/conf.StreamConfig.Build` |
| `FuzzXrayCoreSniffingRoutingDNSConfigBuild` | Sniffing, routing, and DNS fragments | `encoding/json` -> `SniffingConfig.Build`, `RouterConfig.Build`, `DNSConfig.Build`, optional `conf.Config.Build` |
| `FuzzXrayCoreVLESSFirstPacket` | VLESS inbound pre-auth first packet | byte input -> `proxy/vless/encoding.DecodeRequestHeader` with `MemoryValidator` |
| `FuzzXrayCoreVLESSInboundProcessPreAuth` | VLESS inbound pre-auth handler path | byte input -> fake `net.Conn` -> `proxy/vless/inbound.Handler.Process` -> parser/auth/flow/dispatch decision |
| `FuzzXrayCoreVLESSInboundFallbackPreAuth` | VLESS inbound fallback-enabled pre-auth path | byte input -> fake `net.Conn` -> `Handler.Process` with fallback map -> first-buffer parser or fallback reject decision |

## Run

Run seed and regression corpus:

```sh
go test ./fuzz/xraycore -run=Fuzz -count=1
```

Run individual fuzz targets:

```sh
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreFullConfigBuild -fuzztime=30s
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreInboundVLESSConfigBuild -fuzztime=30s
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreOutboundConfigBuild -fuzztime=30s
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreStreamSettingsBuild -fuzztime=30s
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreSniffingRoutingDNSConfigBuild -fuzztime=30s
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreVLESSFirstPacket -fuzztime=30s
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreVLESSInboundProcessPreAuth -fuzztime=30s
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreVLESSInboundFallbackPreAuth -fuzztime=30s
```

For longer local runs, prefer one target per process and set the global timeout explicitly:

```sh
go test ./fuzz/xraycore -run=^$ -fuzz=FuzzXrayCoreVLESSFirstPacket -fuzztime=10m -timeout=15m
```

## Seed corpus

Initial seeds are present in two forms:

1. Programmatic `f.Add` seeds in the fuzz target source files.
2. Persistent Go corpus files under `fuzz/xraycore/testdata/fuzz/<target>/`.

Config seeds include minimal full config, VLESS inbound, VLESS outbound, WebSocket/TLS, gRPC, stream fragments, DNS hosts, routing rules, and near-valid invalid JSON/config samples.

VLESS first-packet seeds include valid TCP/domain, TCP/IPv4, TCP/IPv6, UDP/IPv4, and Mux first packets for UUID `11111111-1111-1111-1111-111111111111`; truncated variants; wrong UUID; bad command; bad address type; malformed domain length; malformed IPv6 payload; oversized addons; XRV flow on raw transport; unknown protobuf flow; and valid prefix plus garbage suffix.

Stage 2 also adds persistent corpora for `FuzzXrayCoreVLESSInboundProcessPreAuth` and `FuzzXrayCoreVLESSInboundFallbackPreAuth`. The fallback harness uses a fallback map that is active for decision-making but intentionally has no matchable default target, so fuzzing reaches fallback selection/reject logic without opening real network connections.

The full-config corpus also contains the minimized known-crash input `{"inBounds":[{"listen":""}]}`. The active fuzz harness quarantines this exact Xray-core empty-domain-listen class so longer runs can continue searching for additional crashes while `TestXrayCoreKnownEmptyListenPanicReproducer` keeps the upstream panic visible.

## Guardrails

The targets cap input size, treat malformed parse/build errors as non-crashing outcomes, fail on unexpected nil successful builds, initialize built full configs with `core.New` where practical, and enforce coarse per-iteration elapsed-time checks. The direct VLESS parser target rejects any successful decode that does not authenticate to the configured seed user or that returns an invalid command/address shape.

The VLESS `Handler.Process` targets use a fake non-blocking `net.Conn` and a recording dispatcher. The oracle fails if malformed, unauthorized, bad-flow, reverse, or structurally incomplete input reaches `DispatchLink`; valid TCP/UDP/Mux seeds must reach `DispatchLink`; and trailing body bytes must not change the parsed first-packet header.
