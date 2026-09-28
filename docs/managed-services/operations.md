# Installation and recovery — incomplete integration

Do not deploy this feature branch as a completed service integration. This
work has not changed any production service, host firewall, qdisc or sshd.

## Development

Use the versions in audit.md, a C compiler, and an isolated checkout:

```sh
go mod download
cd frontend
npm ci
npx playwright install chromium
cd ..
make dist-stub
make test-go
make verify
```

`make dist-stub` only satisfies Go embedding for tests; it is not a usable UI.
`make build` compiles the real frontend and backend. Configure runtime data
under the checkout using .env.example. Use ephemeral ports and loopback for
tests. Privileged network tests must run in a new namespace/container and
refuse to run against the host network namespace.

## Required delivery work

- Pin backend assets and checksum manifests by architecture; capability probe
  before enabling a service. Do not bundle Snell until redistribution terms
  are established. Keep its beta label explicit.
- Update Docker libc/toolchains and expose only selected service listeners.
- Set fork-specific installation/update origin and channel. Reject an update
  that would silently replace the managed-policy build with upstream binaries.
- Back up DB, source cursors, billing remainders, policy revisions, host keys,
  managed service bindings and backend manifest. Secret backups require the
  same protection as the current panel database.
- On restore, stop managed data paths, restore atomically, rotate source
  incarnations/leases under the restored billing authority, reconcile policy
  state, then allow traffic. Never attach an old counter to a recreated user.
- Rollback with a tested pre-upgrade complete backup and matching binaries.
  No safe downgrade migration has been established. Do not run an older
  binary over a partially migrated policy database or discard billing state.
- Uninstall cleans only project-owned resources; never flush the host ruleset,
  reset a global qdisc, remove unrelated containers or change management SSH.

These are required implementation/validation tasks. No installation script or
rollback guarantee for the new backends is claimed at this stage.
