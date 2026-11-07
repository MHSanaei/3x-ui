# 3X-UI version layout

The repository keeps three published versions with separate purposes:

| Version | Type | Purpose | Update path |
| --- | --- | --- | --- |
| `v3.7.0-xnvgatebox-pool.1` | Historical prerelease | Previous fixed xnvgatebox/exit-pool build | Install its release tag explicitly |
| `dev-latest` | Rolling prerelease | Panel-compatible Dev update channel; currently tracks the IPv6/DNS hardening build | Panel Dev channel or `x-ui update-dev` |
| `3x-ui-20261107` | Permanent dated prerelease | Auditable snapshot of the IPv6/DNS hardening build and its checksums | Install the dated release asset explicitly |

Source branches are retained separately:

- `3x-ui-original-20261107` preserves `main` before the IPv6/DNS changes.
- `3x-ui-20261107` preserves the reviewed dated source snapshot.
- `main` is the active development branch that publishes `dev-latest`.

The normal stable updater must not be assumed to select a prerelease. Use the
Dev channel for panel updates to this build. Before changing versions, back up
`/etc/x-ui/x-ui.db`, `/usr/local/x-ui`, `/usr/bin/x-ui`, and any external
certificate or environment configuration. Detailed commands are in
[`upgrade-rollback-3x-ui-20261107.md`](upgrade-rollback-3x-ui-20261107.md).

