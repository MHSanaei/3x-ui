# 12. Change Management and Rollout

## Goal

Provide low-risk patterns for introducing protocol/config/UI/backend changes in a running panel.

## Safe rollout pattern

1. Backup DB.
2. Add new config in parallel (do not replace old path immediately).
3. Validate with one test client.
4. Migrate a small user slice.
5. Monitor logs/latency/failure rate.
6. Complete migration and decommission old path.

## For inbound/protocol changes

Do first:
- Naming cleanup and explicit role labels.
- New test inbound with clear `-test` suffix.

Do later:
- Security/transport rotations on production inbound.

## For client policy changes

- Apply policy in additive way.
- Prefer staged updates by user segment.
- Confirm no accidental disable due to quota/expiry mismatch.

## For custom feature changes

- Keep additive APIs; avoid breaking old endpoints.
- Feature-flag or route-gate where possible.
- Maintain backward compatibility with inbound-native operations.

## Regression checklist

After any significant change verify:
1. Login and panel navigation.
2. Inbounds list and edit flow.
3. Add/update/delete client flow.
4. Export URL/subscription flow.
5. Xray restart and log health.
6. Custom clients page CRUD + sync.

## Rollback checklist

1. Stop rollout.
2. Revert to known-good binary/config.
3. Restore DB backup if data mutation is root cause.
4. Re-validate panel and xray state.
5. Reintroduce change in smaller scope.
