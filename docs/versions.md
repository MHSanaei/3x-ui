# 3X-UI version layout

The repository gives every new build its own dated release. The
`3x-ui-20261107` assets were replaced in place only for the explicitly
requested updater-lock and stable-channel corrections.

| Version                    | Type                         | Purpose                                                                                | Update path                                                                                      |
| -------------------------- | ---------------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `v3.7.0-xnvgatebox-pool.1` | Historical prerelease        | Previous fixed xnvgatebox/exit-pool build                                              | Install its release tag explicitly                                                               |
| `dev-6d205a91-backup`      | Permanent rollback backup    | Byte-for-byte preservation of the known-good `dev+6d205a91` build                      | Install this backup tag explicitly                                                               |
| `dev-latest`               | Compatibility update channel | Carries the currently recommended dated stable assets for historical Dev installations | Existing Dev panels can update directly from the panel                                           |
| `3x-ui-20261107`           | Latest stable dated release  | IPv6/DNS hardening plus update-lock recovery and stable dated-version detection        | Install once explicitly when migrating from frozen Dev; later dated releases appear in the panel |

Source branches are retained separately:

- `3x-ui-original-20261107` preserves `main` before the IPv6/DNS changes.
- tag `3x-ui-20261107` points to the reviewed stable source snapshot.
- `main` is the active development branch. New builds are published under
  explicit `3x-ui-<date>` tags; a successful dated release also refreshes
  the `dev-latest` compatibility alias without modifying the permanent backup.

The stable updater follows GitHub's latest non-prerelease and compares
`3x-ui-YYYYMMDD` identities. Historical `dev+6d205a91` installations see
the current dated stable build through `dev-latest`, so they can migrate with
the panel update button. After that, later dated releases are offered through
the normal stable channel. Before changing versions, back up
`/etc/x-ui/x-ui.db`, `/usr/local/x-ui`, `/usr/bin/x-ui`, and any external
certificate or environment configuration. Detailed commands are in
[`upgrade-rollback-3x-ui-20261107.md`](upgrade-rollback-3x-ui-20261107.md).
