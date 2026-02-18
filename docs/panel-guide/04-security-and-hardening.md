# 04. Security and Hardening

## Immediate high-priority items

1. Enable TLS for panel access.
2. Change default/guessable panel base path.
3. Change default subscription paths.
4. Use strong admin password + 2FA.
5. Restrict panel listen IP where possible.

## Operational hardening

- Keep backups of DB before major changes.
- Use staged config changes, not bulk edits.
- Keep one known-good inbound active.
- Review logs after each restart.

## Control-plane warning handling

If panel shows security warning banner:
- Treat as real risk, not cosmetic.
- Do not expose panel publicly without TLS.

## Inbound safety rules

For active user inbounds:
- Avoid sudden port/security/transport changes.
- Avoid key/shortId rotation without migration window.
- Avoid disable/delete on active inbounds without user communication.

Safe changes anytime:
- Remark/naming cleanup
- Client naming consistency
- Non-functional labeling and grouping

## Current naming standard recommendation

Use:
- `<protocol>-<transport>-<security>-<port>-<role>`

Examples:
- `vless-reality-tcp-443-main`
- `vless-reality-tcp-8443-alt`
- `vless-tcp-http-18080-test`

## Suggested maintenance cadence

Daily:
- Check Xray state, error logs, traffic anomalies

Weekly:
- Review depleted/disabled clients
- Validate backup and restore path

Monthly:
- Rotate sensitive paths/credentials if needed
- Review exposed interfaces and firewall rules
