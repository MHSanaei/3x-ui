# 07. Dev and Testing Workflow

## Local dev philosophy

Keep all local mutable state inside repo `tmp/`:
- `tmp/db`
- `tmp/logs`
- `tmp/bin`
- `tmp/cookies`

## Fast dev run (without air)

```bash
mkdir -p tmp/db tmp/logs
XUI_DB_FOLDER="$PWD/tmp/db" XUI_LOG_FOLDER="$PWD/tmp/logs" XUI_DEBUG=true \
  go run . setting -port 2099 -username admin -password admin
XUI_DB_FOLDER="$PWD/tmp/db" XUI_LOG_FOLDER="$PWD/tmp/logs" XUI_DEBUG=true \
  go run . run
```

Panel:
- URL: `http://127.0.0.1:2099`
- Login: `admin/admin` (fresh DB)

## Air live-reload

Config file:
- `.air.toml`

Run:
```bash
air -c .air.toml
```

Behavior:
- Builds to `tmp/bin/3x-ui-dev`
- Runs with tmp db/log env vars
- Bootstraps fresh DB credentials/port when DB absent

## Justfile commands

- `just init-dev`
- `just run`
- `just air`
- `just build`
- `just check`
- `just api-login`
- `just api-clients-inbounds`
- `just api-clients-list`
- `just clean-tmp`

## Validation checklist for custom clients feature

1. Login and open `/panel/clients`.
2. Create master client with one inbound assignment.
3. Confirm row appears in clients table.
4. Confirm assignment visible in list API.
5. Update policy fields and assignment set.
6. Confirm synced result in list API.
7. Delete master client and confirm list is empty.

## Notes from local test run

- Build and startup validated on local SQLite paths.
- API + UI flows for custom clients feature validated.
- Xray API nil-client panic path was fixed and replaced with explicit errors.
