# 15. Remnawave + Hiddify Analysis

## Purpose

This document captures features from `remnawave/panel` and `hiddify/Hiddify-Manager` that can inspire our custom `3x-ui`, with focus on low-breakage adoption.

## Remnawave (panel) Findings

### Product shape

- TypeScript-oriented ecosystem (panel/backend/frontend/node tooling around Xray).
- Strong operator UX around users, nodes, templates, routing rules, squads, and notifications.

### Standout capabilities

- Squad model: group-based control over what users can access.
- Template/rule engine: conditional responses and per-client template behavior.
- Webhooks: event delivery for user/node/service/error events.
- Notification tuning: per-channel per-event control.
- HWID device limits and device history/management.
- SDK and automation orientation for external integrations.

### What to borrow first

1. Event webhooks in `3x-ui` for user/client/inbound lifecycle changes.
2. Notification matrix (events x channels) rather than global on/off.
3. Device management UI for client anti-sharing controls.
4. A simplified policy-group model (Remnawave squads inspiration) for assigning many inbounds to many clients cleanly.

## Hiddify-Manager Findings

### Product shape

- Operationally heavy Linux stack: panel + generated configs + many services.
- Template generation pipeline for Xray/Sing-box/HAProxy/nginx and helper services.

### Standout capabilities

- Broad protocol and transport coverage (including newer stacks).
- HAProxy/nginx map-based dispatch and multiplexing.
- Auto-update/backup and operational scripts.
- Extra operator tooling (short links, helper pages, bot integration).

### What to borrow first

1. Template-first config generation mindset for complex transport combinations.
2. Optional HAProxy/nginx routing templates as an advanced deployment profile.
3. Operational guardrails: backup/update helpers and health checks.

## Fit For Custom 3x-ui

### Low-risk, high-value (phase 1)

1. Webhook events and signed delivery.
2. Notification preference matrix.
3. Better client detail pages (usage, devices, assignment visibility).

### Medium effort (phase 2)

1. Policy groups for inbound bundles and client assignment.
2. Template/rule editor for client-aware output behavior.
3. More advanced export/subscription templates.

### High effort / platform-level (phase 3)

1. Full map-based edge routing orchestration (HAProxy/nginx style).
2. Multi-core orchestration parity (Xray + Sing-box in one control plane).
3. Full installer-grade lifecycle automation comparable to Hiddify.

## Recommended Direction

Prefer **Remnawave-inspired UX/control-plane features first** because they map well to our current custom `3x-ui` UI/backend extension path.

Adopt **Hiddify-inspired ops architecture selectively** as optional deployment modules, not as core assumptions for all users.

## Safe Implementation Rules

1. Add each feature behind a feature flag.
2. Keep backward compatibility for existing inbound/client behavior.
3. Add migration scripts with rollback steps.
4. Ship contract tests for webhook payloads and assignment logic.
5. Validate new behavior in local sqlite dev + one staging VPS before production rollout.
