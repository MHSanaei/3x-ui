# 16. Unified Product Roadmap (3x-ui Custom)

## Objective

Build a production-grade custom `3x-ui` that keeps current compatibility while adding modern client management, automation, policy control, and optional advanced ops capabilities inspired by Marzban, Remnawave, and Hiddify.

## Source of ideas

- `3x-ui` baseline: inbound-centric model, compatibility, existing operator workflows.
- Marzban inspiration: node architecture, webhook/admin automation, host/profile concepts.
- Remnawave inspiration: squads/policy groups, template/rule engine, notification matrix, HWID management.
- Hiddify inspiration: template-driven config generation, advanced edge routing, strong operational tooling.

## Product principles

1. Backward-compatible by default.
2. Additive changes first, migrations only with rollback.
3. Feature flags for every major module.
4. Safe defaults for exposure, auth, and transport settings.
5. Operator-first UX: fast bulk actions, clear observability, low-click routine tasks.

## Current baseline (already done)

- Centralized client page exists (`/panel/clients`) with master client + inbound assignment sync.
- Local dev tooling exists (`justfile`, `air`, sqlite dev flow).
- Documentation split exists under `docs/panel-guide`.

## Roadmap phases

## Phase 0: Stabilize Foundation (1 week)

### Deliverables

1. Harden current client-center behavior.
2. Add regression tests for assignment/sync paths.
3. Add lightweight structured audit for client mutations.

### Acceptance criteria

1. No regression in existing inbound CRUD and client operations.
2. Sync errors are user-visible and recoverable.
3. Automated tests cover add/update/delete + attach/detach flows.

## Phase 1: Operator Productivity Core (2-3 weeks)

### Deliverables

1. Plan presets (`30d-100GB`, `90d-300GB`, `unlimited`) with one-click apply.
2. Bulk client actions: extend expiry, add/reset quota, enable/disable, tag/comment edits.
3. Better client details: assigned inbounds, usage summary, expiry status, quick actions.
4. Audit log UI/API for sensitive changes.

### Inspiration mapping

- Marzban: plan-like lifecycle and operational workflows.
- Remnawave: stronger user detail and admin ergonomics.

### Acceptance criteria

1. Operator can modify 100+ clients via previewed bulk action safely.
2. Every sensitive operation writes immutable audit records.
3. Average clicks for common tasks reduced (create client, assign inbounds, extend plan).

## Phase 2: Automation and Integration (2 weeks)

### Deliverables

1. Webhook endpoints with per-event filtering and HMAC signatures.
2. Delivery logs, retry policy, and test-send action.
3. Notification matrix (event x channel) with sane defaults.

### Inspiration mapping

- Remnawave: webhook scopes + fine-grained notification tuning.
- Marzban: external integration-friendly operator model.

### Acceptance criteria

1. Webhook payload contracts are versioned and documented.
2. Failed deliveries are visible with retry status.
3. Operators can disable noisy events without disabling all alerts.

## Phase 3: Policy Groups and Access Model (2-3 weeks)

### Deliverables

1. Policy groups (squad-like) to assign bundles of inbounds/settings to clients.
2. Group-level limits/metadata inheritance with per-client override.
3. Group-based bulk assignment and membership management UI.

### Inspiration mapping

- Remnawave: Internal/External squad model.

### Acceptance criteria

1. Many-to-many assignment handled without manual per-inbound editing.
2. Inheritance and override behavior is deterministic and documented.
3. Migration from existing direct assignment remains seamless.

## Phase 4: Template and Subscription Engine (3-4 weeks)

### Deliverables

1. Host/profile template manager with preview variables.
2. Output templates by client/app/core (xray/sing-box/mihomo style profiles).
3. Rule-based response behavior (safe subset first): header/app-based template selection.

### Inspiration mapping

- Remnawave: templates + response rules.
- Marzban: host/profile customization model.

### Acceptance criteria

1. Operators can preview rendered output before publishing.
2. Template changes are versioned and rollbackable.
3. Existing subscription links continue working.

## Phase 5: Security and Anti-Abuse Layer (2-3 weeks)

### Deliverables

1. Device tracking and optional HWID/device limits per client.
2. Risk controls: suspicious device churn alerts, quick revoke flows.
3. Policy options for account sharing tolerance level.

### Inspiration mapping

- Remnawave: HWID/device management and anti-sharing controls.

### Acceptance criteria

1. Device history is queryable per client.
2. Operators can enforce or relax limits per policy group.
3. Enforcement failures fail safe and are auditable.

## Phase 6: Advanced Ops Profile (Optional, staged, 4-8 weeks)

### Deliverables

1. Template-driven advanced config generation module.
2. Optional edge routing profile (HAProxy/nginx map-based patterns).
3. Optional multi-core orchestration profile (xray + sing-box).
4. Backup/update/health-check operational toolkit.

### Inspiration mapping

- Hiddify: config generation and ops automation.

### Acceptance criteria

1. Advanced profile is optional and isolated from default path.
2. Health checks and rollback mechanisms are mandatory.
3. Security review passes before enabling in production.

## Cross-cutting workstreams

## Data model and migration safety

1. Every schema change gets forward + rollback migration plan.
2. Soft launch new tables/columns before hard dependencies.
3. Keep legacy APIs until replacement is validated.

## API design and compatibility

1. Version new APIs under clear prefixes when behavior differs.
2. Keep old endpoint behavior stable unless explicitly deprecated.
3. Add contract tests for webhook/template/assignment APIs.

## UI/UX consistency

1. Preserve current mental model for existing users.
2. Expose advanced controls progressively (basic vs advanced tabs).
3. Add inline docs/tooltips for protocol-sensitive fields.

## Testing and quality gates

1. Unit tests for model/service logic.
2. Integration tests for db migrations and sync flows.
3. UI flow tests (Playwright) for client lifecycle and bulk operations.
4. Staging soak test before each phase release.

## Security controls

1. HMAC for outbound webhooks.
2. RBAC prep for future multi-admin model.
3. Secret rotation support for API tokens and webhook secrets.
4. Harden defaults for exposed panels and management ports.

## Release strategy

1. Release per phase behind feature flags.
2. Rollout order: local sqlite -> staging VPS -> production VPS.
3. Each release ships with:
   - migration notes
   - rollback procedure
   - updated docs in `docs/panel-guide`
   - smoke test checklist

## Success metrics

1. Time to create + assign a client drops by at least 50%.
2. Bulk operations complete with zero silent failures.
3. Webhook delivery success rate stays above 99% after retries.
4. Support incidents caused by misconfiguration trend downward.
5. No increase in critical outages after enabling new modules.

## Suggested execution order

1. Phase 0 + Phase 1 immediately.
2. Phase 2 next (automation unlock).
3. Phase 3 and Phase 4 together only after Phase 1/2 are stable.
4. Phase 5 once policy groups are in place.
5. Phase 6 only as opt-in advanced deployment profile.

## Non-goals (for now)

1. Full rewrite of 3x-ui core architecture.
2. Mandatory multi-core or edge-router stack for all users.
3. Breaking changes to existing inbound-native workflows.
