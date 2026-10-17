# Active client profiles

The clients table and mobile card display the names of inbounds with live authenticated dispatcher links for that client. Shared credentials can have several profiles active at once, including on different nodes. Attribution uses client email, physical node GUID and inbound tag together; a common source IP or traffic on an unrelated inbound is not sufficient evidence.

## Core capability and compatibility

The companion Xray change adds `UserStat.inbound_tags` (field 4) and `GetUsersStatsResponse.inbound_tracking` (field 2). Its separate online maps follow the existing link cancellation and reference-counting lifetime. Existing aggregate user/IP/traffic counters retain their semantics. This panel change does not alter the upstream Xray Go module dependency pin: the compatibility reader handles both protobuf unknown fields with the current dependency and declared fields after a future dependency update.

The stock core remains usable and produces an unknown state in the new column. A supported empty snapshot means no open authenticated links, not a collection failure. The view tracks opened links, not a profile selected in an app before any connection; it does not identify individual people sharing an account. Kernel-native/unsupported tunnel implementations are not inferred from other counters.

## API and node synchronization

Authenticated `POST /panel/api/clients/activeProfiles` returns the usual `success/obj` envelope. `obj`, also present as `activeProfiles` in traffic WebSocket updates, maps each physical node GUID to:

```json
{
  "supported": true,
  "observedAt": 1792180000000,
  "clients": { "alice": ["in-vless", "in-hysteria"] }
}
```

Admin sessions/tokens and node-sync tokens may read it; monitor tokens may not. No credentials, source IPs or destinations are included. The feature is memory-only: no database schema changes, access logs or connection history are needed.

Snapshots fully replace earlier state. Local receipt TTL is 20 seconds. Relays preserve the source timestamp, and competing paths to the same physical GUID select the newest snapshot instead of unioning stale connections. Node selection, tag aliases and cloned GUIDs follow the existing sync rules. Legacy/unreachable collectors yield unknown state. The frontend uses one aggregate query and the existing traffic WebSocket, not a request per client row.

## Tests

Run the normal Go and frontend checks without an unmerged/forked Go dependency. `internal/xray/online_inbound_proto_test.go` covers both descriptor versions, absent capability and malformed metadata. Other regression tests cover disconnects, stale state, deep copies, node identity/tag remapping, same-IP client isolation and authorization scopes. Storybook includes synthetic concurrent/disconnected/unknown fixtures under `Clients/ClientActiveProfiles`.

The optional real-core integration test needs an Xray executable with the companion change:

```bash
XRAY_E2E_BINARY=/absolute/path/to/patched/xray go test -race ./internal/xray -run TestXrayAPI_E2E_ActiveProfiles -count=1 -v
```

It opens local VLESS and Hysteria2 tunnels, checks simultaneous profiles and same-source-IP isolation, keeps an idle link open and then verifies removal on disconnect. The Hysteria test validates its locally issued certificate; it does not bypass TLS checks. Without the executable, the optional test skips explicitly.
