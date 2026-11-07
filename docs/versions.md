# 3X-UI version layout

The repository keeps three published versions with separate purposes:

| Version | Type | Purpose | Update path |
| --- | --- | --- | --- |
| `v3.7.0-xnvgatebox-pool.1` | Historical prerelease | Previous fixed xnvgatebox/exit-pool build | Install its release tag explicitly |
| `dev-latest` | Frozen historical build | Permanently preserves `dev+6d205a91`; it is never overwritten by later releases | Panel Dev channel or `x-ui update-dev` installs this historical build |
| `3x-ui-20261107` | Permanent dated prerelease | Auditable snapshot of the IPv6/DNS hardening build and its checksums | Install the dated release asset explicitly |

Source branches are retained separately:

- `3x-ui-original-20261107` preserves `main` before the IPv6/DNS changes.
- `3x-ui-20261107` preserves the reviewed dated source snapshot.
- `main` is the active development branch. It does not publish or move
  `dev-latest`; new builds are published only under explicit `3x-ui-<date>`
  version tags.

The normal stable updater must not be assumed to select a prerelease. The Dev
channel intentionally returns the frozen `dev+6d205a91` build; install newer
dated releases explicitly. Before changing versions, back up
`/etc/x-ui/x-ui.db`, `/usr/local/x-ui`, `/usr/bin/x-ui`, and any external
certificate or environment configuration. Detailed commands are in
[`upgrade-rollback-3x-ui-20261107.md`](upgrade-rollback-3x-ui-20261107.md).

