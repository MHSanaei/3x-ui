# 3X-UI version layout

The repository gives every new build its own dated release. The
`3x-ui-20261107` assets were replaced in place only for the explicitly
requested updater-lock and stable-channel corrections.

| Version | Type | Purpose | Update path |
| --- | --- | --- | --- |
| `v3.7.0-xnvgatebox-pool.1` | Historical prerelease | Previous fixed xnvgatebox/exit-pool build | Install its release tag explicitly |
| `dev-latest` | Frozen historical build | Permanently preserves `dev+6d205a91`; it is never overwritten by later releases | Panel Dev channel or `x-ui update-dev` installs this historical build |
| `3x-ui-20261107` | Latest stable dated release | IPv6/DNS hardening plus update-lock recovery and stable dated-version detection | Install once explicitly when migrating from frozen Dev; later dated releases appear in the panel |

Source branches are retained separately:

- `3x-ui-original-20261107` preserves `main` before the IPv6/DNS changes.
- tag `3x-ui-20261107` points to the reviewed stable source snapshot.
- `main` is the active development branch. It does not publish or move
  `dev-latest`; new builds are published only under explicit `3x-ui-<date>`
  version tags.

The stable updater follows GitHub's latest non-prerelease and compares
`3x-ui-YYYYMMDD` identities. The Dev channel intentionally returns the
frozen `dev+6d205a91` build, so that historical binary needs one explicit
installation of `3x-ui-20261107` to migrate to the stable dated channel.
After that, later dated releases are offered in the panel. Before changing
versions, back up
`/etc/x-ui/x-ui.db`, `/usr/local/x-ui`, `/usr/bin/x-ui`, and any external
certificate or environment configuration. Detailed commands are in
[`upgrade-rollback-3x-ui-20261107.md`](upgrade-rollback-3x-ui-20261107.md).
