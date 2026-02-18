# 14. Marzban-Inspired Roadmap (Implementation Draft)

## Goal

Borrow high-impact ideas from Marzban and implement them safely in custom 3x-ui without breaking existing inbound-native workflows.

## Reference baseline

This draft is based on Marzban concepts visible in official repo/docs (node architecture, host settings, webhooks, CLI/admin flows), adapted to current 3x-ui architecture and your implemented client-center extension.

---

## Phase 1 (fast, low-risk, high ROI)

### 1) Plan Presets (User Templates)

Purpose:
- Reusable client plans (for example: `30d-100GB`, `90d-300GB`, `Unlimited`).

DB:
- `plan_presets`:
  - `id`, `user_id`, `name`, `total_gb`, `duration_days`, `limit_ip`, `enable`, `comment`, `created_at`, `updated_at`

API:
- `GET /panel/api/plans/list`
- `POST /panel/api/plans/add`
- `POST /panel/api/plans/update/:id`
- `POST /panel/api/plans/del/:id`

UI:
- Add `Plans` tab/page.
- In `/panel/clients`, add “Apply Plan” selector for create/edit.

Acceptance:
- Create plan once and apply to new/existing master clients in one click.

---

### 2) Webhook Events

Purpose:
- External automation (billing, CRM, alerts) decoupled from panel internals.

DB:
- `webhook_endpoints`:
  - `id`, `user_id`, `name`, `url`, `secret`, `enabled`, `event_filter_json`, `created_at`, `updated_at`
- `webhook_deliveries`:
  - `id`, `endpoint_id`, `event`, `payload_json`, `status_code`, `success`, `error`, `created_at`

Events (initial):
- `master_client.created`
- `master_client.updated`
- `master_client.deleted`
- `assignment.created`
- `assignment.updated`
- `assignment.deleted`
- `inbound.client.sync_failed`

API:
- `GET /panel/api/webhooks/list`
- `POST /panel/api/webhooks/add`
- `POST /panel/api/webhooks/update/:id`
- `POST /panel/api/webhooks/del/:id`
- `GET /panel/api/webhooks/deliveries`
- `POST /panel/api/webhooks/test/:id`

Implementation note:
- HMAC signature header with endpoint secret.
- Non-blocking async delivery queue + retry policy.

Acceptance:
- Webhook test succeeds and deliveries are visible/auditable.

---

### 3) Host/Profile Settings Templates

Purpose:
- Better subscription/profile text rendering with variables.

DB:
- `host_templates`:
  - `id`, `user_id`, `name`, `template_text`, `enabled`, `created_at`, `updated_at`

Template variables (v1):
- `{{master_name}}`
- `{{email_prefix}}`
- `{{inbound_remark}}`
- `{{protocol}}`
- `{{port}}`
- `{{expiry_time}}`
- `{{days_left}}`
- `{{total_gb}}`
- `{{used_gb}}`
- `{{remaining_gb}}`

API:
- `GET /panel/api/host-templates/list`
- `POST /panel/api/host-templates/add`
- `POST /panel/api/host-templates/update/:id`
- `POST /panel/api/host-templates/del/:id`
- `POST /panel/api/host-templates/preview`

UI:
- `Settings -> Subscription` add template manager/preview.

Acceptance:
- Template preview and rendered subscription info align with selected inbound/client.

---

## Phase 2 (medium effort, operational depth)

### 4) Bulk Policy Actions

Purpose:
- Apply +days/+GB/enable/disable/comments to selected master clients.

API:
- `POST /panel/api/clients/bulk`
  - filters: ids, tags, enabled state
  - operations: set/adjust fields

Safety:
- Dry-run mode (`preview=true`) returns mutation summary.

Acceptance:
- Admin can bulk-modify 100+ clients with preview and audit trail.

---

### 5) Audit Log

DB:
- `audit_logs`:
  - actor, action, target_type, target_id, before_json, after_json, created_at

Coverage:
- Client-center, plan, webhook, template, inbound sync actions.

Acceptance:
- Every sensitive action has immutable audit row.

---

## Phase 3 (advanced, higher risk)

### 6) Multi-admin Ownership / RBAC

Roles:
- `owner`, `admin`, `operator`, `viewer`

Scope:
- Per-user ownership of master clients and optional inbound scope controls.

Acceptance:
- Role matrix enforced on APIs and UI actions.

---

### 7) Node Architecture (Marzban-style inspiration)

Purpose:
- Manage remote xray nodes from a central control panel.

Model (initial):
- Central panel keeps desired state.
- Node agent pulls signed config deltas + pushes health/traffic.

Major components:
- `nodes` table and node auth keys
- Node heartbeat + capability registry
- Config distribution queue
- Health and lag monitoring

Acceptance:
- One remote node managed reliably with inbound/client sync and health checks.

---

## Cross-cutting engineering rules

1. Additive-first: keep old APIs/UI working.
2. Feature flags for each new module.
3. Idempotent sync operations.
4. Queue + retries for external IO (webhooks/nodes).
5. Full rollback path (DB backup + binary rollback).

---

## Suggested first implementation sprint (5-7 days)

1. `plan_presets` DB/API/UI
2. Webhook endpoints + delivery log + test button
3. Host template CRUD + preview
4. Minimal audit logs for these three modules

Deliverable at sprint end:
- Operators can provision clients faster, integrate billing/automation, and improve subscription profile quality without architecture-breaking changes.

---

## Fit with current custom code

Already aligned foundations:
- Master client model exists (`MasterClient`, `MasterClientInbound`).
- Central clients API/UI exists (`/panel/clients`, `/panel/api/clients/*`).
- Dev workflow exists (`.air.toml`, `justfile`).

So this roadmap extends current direction directly, no redesign needed for Phase 1.
