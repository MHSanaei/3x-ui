# Stage 1 technical note

## Covered

Config fuzzing covers the Xray-core JSON/config build surface without starting listeners:

- `infra/conf/serial.DecodeJSONConfig` for full JSON config loading.
- `infra/conf.Config.Build` for global config normalization and app/inbound/outbound config construction.
- `core.New` for non-running runtime object initialization after successful full-config builds.
- `InboundDetourConfig.Build` and `VLessInboundConfig.Build` for inbound/VLESS settings, users, decryption, fallback, stream, and sniffing handling.
- `OutboundDetourConfig.Build` and `VLessOutboundConfig.Build` for outbound/VLESS endpoint, user, stream, proxy, and mux handling.
- `StreamConfig.Build` for transport/security fragments: TCP/raw, WS, gRPC, XHTTP, KCP, TLS, REALITY, sockopt, and finalmask paths where reachable from JSON.
- `SniffingConfig.Build`, `RouterConfig.Build`, and `DNSConfig.Build` as focused fragment targets.
- API/stats/metrics are included through full config seeds and `conf.Config.Build`.

VLESS first-packet fuzzing covers the early inbound pre-auth parser directly:

- `proxy/vless/encoding.DecodeRequestHeader`.
- `proxy/vless.MemoryValidator` with one configured valid client UUID.
- Version, raw UUID, addons length/value, command, and destination parser handling.
- Both direct reader mode and the first-buffer mode used by VLESS inbound after the first socket read.

Stage 2 adds handler-level pre-auth coverage:

- `proxy/vless/inbound.(*Handler).Process` first-read path.
- `connection.SetReadDeadline`, first `buf.Buffer.ReadFrom`, `buf.BufferedReader` setup, and parser call.
- UUID lookup through `MemoryValidator`.
- Flow admission for empty flow, unknown flow, and `xtls-rprx-vision` on a raw fake connection.
- Command/destination admission for TCP, UDP, Mux, and Rvs.
- Success handoff into `routing.Dispatcher.DispatchLink` using a recording dispatcher.
- Fallback-enabled first-buffer parser/reject path without dialing a real fallback target.
- Invariants for wrong UUID, malformed address metadata, malformed addon length/value, bad command, bad version, short reads, and valid prefix plus trailing body bytes.

## Bugs found

One Xray-core config-build crash was found by `FuzzXrayCoreFullConfigBuild`:

- Minimal reproducer: `{"inBounds":[{"listen":""}]}`
- Crash: `panic: runtime error: index out of range [0] with length 0`
- Upstream location: `github.com/xtls/xray-core/infra/conf.(*InboundDetourConfig).Build`, `infra/conf/xray.go:152`
- Cause: empty string `listen` is parsed as a domain address with `Domain() == ""`; the build path indexes `Domain()[0]` while checking for Unix domain sockets.

The minimized reproducer is retained in `testdata/fuzz/FuzzXrayCoreFullConfigBuild/85cbe7a11661b2e3`. The active fuzz harness now quarantines this known empty-domain-listen class before calling `Config.Build` so subsequent fuzzing can continue. `TestXrayCoreKnownEmptyListenPanicReproducer` directly calls the upstream build path under `recover` and asserts the panic still reproduces; when Xray-core fixes the bug, that test should fail and the quarantine should be removed.

The deterministic regression test `TestXrayCoreVLESSWrongUUIDRejected` verifies that a packet with a changed non-normalized UUID byte does not authenticate.

No VLESS first-packet parser crash, hang, or false dispatch was found in the Stage 2 smoke-runs.

## Problems encountered

The config loader imports Xray-core serial config support, which requires additional indirect module metadata in the parent module:

- `github.com/ghodss/yaml`
- `github.com/pelletier/go-toml`
- `gopkg.in/yaml.v2`

These are Xray-core config-loader dependencies, not fuzzing infrastructure.

`core.New` is used only after successful build. It initializes Xray runtime objects but does not call `Start`, so it does not bind sockets or run transport lifecycles.

## Not covered in stage 1

- Stateful network harnesses for TCP, WS, gRPC, TLS, REALITY, QUIC, or full VLESS sessions.
- Xray listener accept loops and actual socket deadlines.
- Fallback connection I/O after malformed VLESS first packets.
- Vision/REALITY encrypted first-packet lifecycle beyond the plain request-header parser.
- Long-running resource leak measurement beyond input caps, build/init checks, and elapsed-time guardrails.
- Corpus minimization for Stage 2, because no VLESS failing input was found in smoke-runs.

These are candidates for stage 2 rather than this stage.
