# WatTunnel integration: implementation and validation

Date: 2026-11-06. Working tree based on `b54325dc`; changes are uncommitted. No push, PR, external deployment, host firewall change, or upstream WatTunnel source modification was performed.

## Implemented

- Separate Tunnels route and CRUD/control API; Local/Remote runtime, revisioned desired state, authenticated ownership and persistent deletion tombstones.
- WatTunnel 3.3.0 subprocess integration, direct/reverse TCP/UDP forwarding, ICMP/TCP/UDP/TLS/WSS and availability-based Auto. Last working carrier is retained on disk. Authenticated engine health drives selection; there is no speed benchmark or automatic reverse discovery.
- AES-GCM secrets bound to tunnel GUID, dedicated keyring, private atomic key/config files, separate no-store administrative token export. Stop remains possible when the keyring is unavailable. No plaintext fallback.
- Linux parent-death handling, service UID and minimal ambient capabilities; Windows Job Object and protected ACLs. MTProto shares bounded child stop/wait and Windows lifetime handling.
- Port conflict checks across tunnels, inbounds, derived relay listeners, panel/subscription listeners, node and protocol/address scopes; actual child bind failures have sanitized diagnostics.
- Strict verified TLS for node synchronization. Shared node HTTP redirects cannot downgrade HTTPS to HTTP. Node-sync can only synchronize tunnels and query capabilities, not export tokens or use ordinary control endpoints.
- React Hook Form/Zod/Ant Design/TanStack Query interface, 14 locale dictionaries, generated DTO/OpenAPI/API reference, EN/RU operating guides.
- Signed pinned release downloader and Linux/Windows/Docker packaging with upstream MIT attribution. Dedicated Linux amd64/arm64 and Windows amd64 workflow and isolated namespace lab scripts.

## Upstream and test environment

The local WatTunnel checkout matches stable tag v3.3.0, commit `5c9dedbf1a0f4f0713764fad3443b4848f89122f`. Nine targeted upstream tests had passed during the preceding investigation.

Local integration checks run on macOS arm64, Go 1.27.3 and Node 26.8.1. macOS is a **remote-management platform**, not a bundled WatTunnel runtime target. The local real-process tests use a source-built v3.3.0 engine as a test fixture; production binary signature/version checks are not bypassable by an environment variable. The dedicated CI workflow is configured to verify native releases; signed release verification was also executed locally in Linux containers.

## Completed checks

| Check                                        | Result                                                                                                                                                                                                                     |
| -------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Actual TCP and UDP forwarding                | All five carriers passed direct/reverse on Linux; TCP/UDP/TLS/WSS also passed under the macOS Go race detector                                                                                                             |
| Auto failover and child crash recovery       | Passed in direct and reverse: UDP removed, client selects TCP; killed child is reaped and recovered                                                                                                                        |
| Wrong key, absent peer, actual bind conflict | Passed with real processes; unauthenticated connections never report connected; bind diagnosis does not expose key material                                                                                                |
| Actual Xray cascade                          | Passed: SOCKS → VLESS client → WatTunnel → VLESS inbound → echo service                                                                                                                                                    |
| Secret/ownership/revision/transaction tests  | Passed: encrypted storage, GUID binding, omitted edit key preservation, stale/conflicting owner rejection, concurrent edits, offline intention, rollback/commit failure, tombstone-before-keyring and Stop without keyring |
| Port checks                                  | Passed for TCP/UDP distinction, inbound directions, derived relay ports and panel port changes                                                                                                                             |
| Master/node E2E                              | Full `make node-e2e` passed, 345.585 s; includes real panel processes, pinned TLS proxy, offline edits/deletes and master restart                                                                                          |
| Token fuzz                                   | Passed 31.374 s, 1,152,089 executions, no failure                                                                                                                                                                          |
| PostgreSQL                                   | Disposable PostgreSQL 14 on loopback: existing PostgreSQL/migration tests and tunnel ciphertext/revision/tombstone preservation passed                                                                                     |
| Release assets                               | RSA-signed manifest and SHA-256 verified for Linux amd64, Linux arm64, Windows amd64; Linux files are static ELF binaries                                                                                                  |
| Linux arm64 runtime                          | Official signed binary: all five carriers in direct/reverse, Auto, wrong key, bind failure, Xray cascade passed in Ubuntu 24.04 Docker                                                                                     |
| Linux amd64 runtime                          | Same matrix passed with the official amd64 binary under Docker Desktop emulation; this is not native amd64 hardware evidence                                                                                               |
| Linux privileges                             | Real `/proc` checks passed: non-root child UID, exactly CAP_NET_RAW for ICMP or CAP_NET_BIND_SERVICE for low TCP listen                                                                                                    |
| Linux parent death                           | Passed: abrupt supervisor exit terminates its child; a real Auto/crash recovery test passed again after the final runtime changes                                                                                          |
| Cross compilation                            | Windows amd64 runtime/ACL/Job Object tests compile                                                                                                                                                                         |
| Frontend checks                              | Typecheck, lint, formatter passed; production Vite and Storybook builds passed                                                                                                                                             |
| Documentation                                | Typecheck, lint, formatting and 112 tests passed; docs OpenAPI matches panel OpenAPI                                                                                                                                       |
| Code generation                              | Re-running the normal generator produces identical SHA-256 hashes for every generated artifact                                                                                                                             |
| Dependency scan                              | `make vulncheck`: no reachable vulnerabilities; one advisory in an unused module path                                                                                                                                      |

## Whole-repository gates

The complete Linux arm64 production Docker image built successfully as `xui-wattunnel-panel-test`. One packaging attempt hit a download timeout; the repeated build with shell tracing completed the normal Xray/MTProto/geodata/WatTunnel packaging. A smoke test used a disposable container, private DB and no published ports. It verified capabilities and the bundled signature/version, API creation of both sides, real HTTP forwarding through managed processes, secret-free list responses, token export, Stop surviving container restart, restored forwarding after Start, and runtime-file cleanup after Delete. The container and its anonymous DB volume were removed afterward.

`make verify` was executed and stops at `gen-check`: that target runs `git diff --exit-code` against the Git baseline, so intentional uncommitted generated additions are reported as a difference. Generated files were not staged to hide that result. Reproducibility was checked by comparing hashes before and after regeneration; downstream checks were run directly.

Initial full `make race` completed with no data-race report, but existing TUIC replacement-window and Discord heartbeat tests hit timing failures. Both passed in an isolated repeat. A full ordinary Go run similarly hit existing node-reset/Discord timing failures; those passed in an isolated repeat. Both complete gates subsequently passed with `GOFLAGS=-p=1 make race` and `GOFLAGS=-p=1 make test-go`.

The first full frontend run hit a Vite dependency-optimizer reload in existing Storybook suites. A warmed repeat had one existing client-renewal-help test time out at five seconds. After an isolated repeat, the complete `npm test -- --maxWorkers=1` run passed: **230 files, 2096 tests**, 232.21 seconds.

The additional reverse Auto test initially used a 12-second helper deadline. Upstream reverse dialers back off for up to 30 seconds, so the helper now uses the specified 45-second readiness window. Direct/reverse failover, crash recovery and last-carrier restoration then passed both with the macOS race detector (44.677 s) and with the signed Linux arm64 binary (42.24 s). This was a test-window correction; it did not change production retry policy.

A subsequent Tunnels component suite passed all four tests, including keyboard Tab navigation inside the dialog, focus on the first invalid field, associated error descriptions, Escape dismissal and focus restoration to the opener. These interaction checks use jsdom/user-event with CSS motion disabled; they are not a manual screen-reader audit. No production UI change was needed.

Go lint passed with `golangci-lint v2.15.0`, compiled using the current toolchain. The locally installed v2.13.2 cannot read Go 1.27.3 export data.

## Ubuntu 24.04 laboratory

The user started Docker during implementation. The dedicated Ubuntu 24.04 lab image built successfully, and all measurements below ran inside a disposable container with `--network none`. The lab created two private network namespaces; firewall and qdisc changes stayed inside those namespaces. No host firewall was changed.

The probe sends 2 MiB over TCP forwarding in 32 sequential 64 KiB echo exchanges. Each direction has 20 ms delay and independent configured packet loss. The probe uses a five-second socket timeout and checks a 30-second budget between chunks. Rates count returned payload once, not both directions. These are single samples on a shared machine, not maximum throughput benchmarks or a transport ranking.

| Carrier | 0% loss, Mbit/s |   1% |   5% |           10% |
| ------- | --------------: | ---: | ---: | ------------: |
| TCP     |            9.29 | 3.99 | 1.50 | 0.48, partial |
| UDP     |            9.21 | 3.58 | 2.75 |          2.21 |
| TLS     |           10.39 | 4.57 | 2.05 | 0.10, partial |
| WSS     |            9.77 | 7.37 | 1.59 | 0.46, partial |
| ICMP    |            1.93 | 4.80 | 2.65 |          2.05 |

TCP/TLS/WSS at 10% loss did not complete within the bounded probe. Their rates describe partial transfers; those samples are not successful throughput checks. Engine metrics and client/server CPU time accompany every sample in [the raw results](wattunnel-lab-results.json). For example, the first TCP baseline reported engine RTT 49.05 ms and client/server CPU 0.15/0.20 seconds. Engine counters are cumulative within a process lifetime; reported loss is the engine's value, not the configured qdisc percentage.

All five carriers recovered after link reconnection and either-side restart. Blocking interserver TCP and UDP interrupted TCP/UDP/TLS/WSS; ICMP continued. Taking the link down interrupted all five, as expected. The first run used stats-only readiness and observed an EOF immediately after a peer restart: the engine's `alive` value can lag the usable forwarding path. The lab now waits for both engine health and a successful echo. The complete recovery run then passed. Both original and repeated samples are retained in the raw results (50 + 35 records).

## Not executed here

- Actual Windows Job Object/ACL execution: tests and CI jobs are added and cross-compile, but no native Windows runner or remote CI run was available.
- External two-VPS cascade and DPI behavior: no VPS credentials were provided. Loopback and laboratory success do not establish DPI bypass on a real network.

## Deployment and operational constraints

Use [the English guide](content/docs/en/operations/wattunnel.mdx) or [the Russian guide](content/docs/ru/operations/wattunnel.mdx). Back up the tunnel keyring with the DB before updating/restoring. Install the matching panel package on both nodes; do not update the engine independently.

`wat1:` contains the shared key but has no certificate-pin field. Configure TLS pins separately. Auto transport changes can terminate active connections. Engine counters restart with the process. Metrics stay on loopback; upstream also exposes debug/admin handlers on that listener, so it must not be reverse-proxied publicly.

A synchronized tunnel is pinned to its panel owner and API-token row/mTLS identity. Deleting and recreating the node-sync token with a new identity does not silently transfer ownership. Keep the identity when rotating credentials or recreate the owned tunnel records deliberately. Moving a tunnel between nodes is delete/recreate. Node deletion is refused until its tunnels have been deleted and remote cleanup confirmed.

GRE/IPIP, custom bonding, shaped transports, DPI tuning, Xray inbound creation and subscription Host automation are outside this change.

Suggested PR title: `feat(tunnels): add managed WatTunnel tunnels with multi-node support`.

## Follow-up: manual VPS installation and service capability bounds

While preparing the two-VPS guide, the packaged systemd capability bounding set exposed an installation defect that the earlier Docker default-capability runs had not exercised. Reproducing the old set with `NoNewPrivileges` enabled failed with `chown: operation not permitted` before child startup.

The three packaged service units now retain `CAP_CHOWN`, `CAP_FOWNER`, `CAP_KILL`, `CAP_SETUID` and `CAP_SETGID` in addition to their existing capabilities. These permit private-file ownership/mode updates, credential dropping and termination of a child with a different UID. `CAP_SETPCAP` was not needed. The child still receives only the required networking capabilities.

The actual five-carrier direct/reverse matrix, stop/start, child UID/capability inspection and parent-death test passed with this explicit restricted set and `no-new-privileges` in Alpine and Ubuntu 24.04 containers. This reproduces the capability restriction; it is not a full systemd boot/seccomp integration test. Linux CI now derives the Docker test bounding set from the packaged service unit, so removing a required capability exercises this failure again.

The [two-VPS guide](content/docs/ru/operations/wattunnel-two-vps.mdx) includes a static CGO build, signed WatTunnel assets, a service-account/drop-in installation, consistent RF/exit port mappings and DB-aware rollback. Static native VPS packages were not produced by this documentation step; choose the architecture from the actual VPS.
