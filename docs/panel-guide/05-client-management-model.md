# 05. Client Management Model (Native 3x-ui)

## Native model truth

3x-ui is inbound-first.
- Clients are stored inside each inbound.
- There is no native global user object that auto-attaches to multiple inbounds.

## Practical centralized behavior (without custom feature)

You can simulate central management by:
1. Reusing the same user identifier pattern (email/name).
2. Applying same policy per inbound client (quota/expiry/ip limit).
3. Automating updates with panel APIs.

## Why model works this way

Core internals are tied to inbound-scoped records:
- Client traffic links to inbound and email
- Client IP tracking is email-based
- Inbound settings embed client arrays

Key files:
- `web/service/inbound.go`
- `xray/client_traffic.go`
- `database/model/model.go`

## Per-client controls

For each inbound client you can set:
- `totalGB`
- `expiryTime`
- `limitIp`
- `enable`
- `comment`
- (protocol-specific identity fields)

## Automation endpoints commonly used

- `POST /panel/api/inbounds/addClient`
- `POST /panel/api/inbounds/updateClient/:clientId`
- `GET /panel/api/inbounds/getClientTraffics/:email`

## Operational recommendation

Use policy templates (for example 30d/100GB, 30d/300GB, unlimited) and apply them consistently.
