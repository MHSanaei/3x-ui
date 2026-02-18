# 11. Troubleshooting Runbook

## Fast triage order

1. Confirm panel reachability and login.
2. Check Xray running state on Overview.
3. Check recent panel logs and Xray logs.
4. Verify inbound enable state and port binding.
5. Verify client enabled/quota/expiry/IP limits.
6. Verify routing and DNS policy impact.

## Common issues

### Panel loads but auth/session fails

Checks:
- Session settings and secret availability.
- Cookie persistence and base path correctness.

Files:
- `web/web.go`
- `web/service/setting.go`

### Inbound saved but traffic not flowing

Checks:
- Runtime apply result (`needRestart` scenarios).
- Xray state and error logs.
- Transport/security mismatch with client app.

Files:
- `web/service/inbound.go`
- `web/service/xray.go`

### Clients unexpectedly disabled/depleted

Checks:
- Auto disable/renew logic from periodic jobs.
- `totalGB`, `expiryTime`, reset behavior.

Files:
- `web/service/inbound.go`
- `web/job/xray_traffic_job.go`

### Assignment sync errors in custom clients feature

Checks:
- Inbound protocol supports multi-client.
- Assignment detach is not removing last remaining client from inbound.
- Underlying inbound client key (id/password/email) still valid.

Files:
- `web/service/client_center.go`
- `web/service/inbound.go`

### Panic or 500 during Xray API interaction

Current guard:
- `xray/api.go` now returns explicit error when handler client is nil.

Action:
- Confirm Xray API availability and startup timing.

## Local dev troubleshooting

If app cannot write default paths:
- Use repo-local env:
  - `XUI_DB_FOLDER=$PWD/tmp/db`
  - `XUI_LOG_FOLDER=$PWD/tmp/logs`

If dependency proxy blocks build:
- `GOPROXY=direct go build ./...`

If login fails in fresh local DB:
- Run init:
  - `go run . setting -port 2099 -username admin -password admin`

## Evidence collection checklist

Collect before major debugging:
1. Active inbounds list JSON.
2. Client traffic record snapshot.
3. Current panel settings export.
4. Recent xray log excerpts around failure time.
