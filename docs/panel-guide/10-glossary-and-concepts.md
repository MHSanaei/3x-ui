# 10. Glossary and Concepts

## Core terms

- Inbound:
  Server-side listener definition (protocol, port, stream/security settings, and clients).

- Outbound:
  Egress route object used by routing policy.

- Client (native 3x-ui):
  Per-inbound user entry embedded under inbound settings.

- Master Client (custom feature):
  Central profile that syncs policy to assigned inbound clients.

- Assignment:
  Link between one master client and one inbound (custom table-backed mapping).

- Stream/Transport:
  Wire transport settings (tcp/ws/grpc/etc) independent from protocol identity.

- Security layer:
  `none`, `tls`, or `reality` depending on stream configuration.

- Sniffing:
  Metadata/domain extraction behavior for routing logic.

- Traffic reset:
  Reset policy cycle for usage counters.

- `needRestart`:
  Service flag indicating runtime API update failed or restart is required for consistency.

## Data model concepts

- `inbounds.settings` stores client arrays as JSON.
- `xray.ClientTraffic` stores usage/state counters per tracked client email.
- `InboundClientIps` stores email-to-IP observations.
- Custom extension adds:
  - `MasterClient`
  - `MasterClientInbound`

## Control plane vs data plane

- Control plane:
  Panel UI/API, DB, settings, orchestration jobs.

- Data plane:
  Xray runtime handling proxy traffic.

Operationally:
- A panel save may succeed in DB while runtime application may require restart.
- Always verify both DB state and runtime state for critical changes.

## Practical mental model

1. Configure inbound protocol/transport/security.
2. Attach clients (native or via master sync).
3. Validate routing/DNS/outbound path.
4. Confirm runtime apply and traffic counters.
