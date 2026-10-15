# Per-client HostGroup assignments

## Purpose

Allow each client to receive only selected HostGroups in its generated subscriptions, without changing the client's inbound attachments or Xray runtime configuration.

## Data model

The `client_hosts(client_id, group_id)` relation stores explicit per-client assignments. Assignments target the stable logical `HostGroup.group_id`, not a physical Host row ID; a HostGroup can contain rows for multiple inbounds and its rows may be recreated during edits.

The `client_group_hosts(group_name, host_group_id)` relation supports defaults assigned to an existing client group. A client-specific assignment takes precedence over its client-group assignment.

## Compatibility and selection

- No individual or applicable client-group assignment: retain legacy behavior and include all enabled, applicable Hosts for the client's inbound.
- One or more assigned HostGroups: include only enabled Hosts belonging to those groups.
- An explicit assignment with no matching Host on an inbound: emit no link for that inbound; do not fall back to the full Host list.
- Clearing an assignment restores legacy behavior.

The existing client-to-inbound membership remains the runtime/Xray attachment layer and is not changed implicitly by Host assignment.

## Subscription integration

The selection is applied at the Host endpoint boundary, while the existing protocol-specific renderers continue producing raw links, JSON and Clash output. Selection must be scoped to the client's attached inbounds and remain isolated between clients.

## Validation

Assignments are normalized to remove blank and duplicate IDs and are validated against known HostGroups before being persisted. Tests should cover set/clear, client-group lifecycle, bulk creation, case-insensitive client lookup, restricted selection, and no-fallback behavior when an assigned group has no Host on an inbound.
