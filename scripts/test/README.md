# IP certificate regression tests

Run from the repository root in Bash. The suite uses temporary files, mocks ACME
orders and service commands, and generates local self-signed certificate fixtures.
It never requests a certificate from a CA or executes the complete installer.

```bash
bash -n install.sh
bash -n x-ui.sh
bash scripts/sync-ip-ssl-helpers.sh --check
bash scripts/test/ip-ssl-test.sh
bash scripts/test/ip-ssl-entrypoint-test.sh
IP_SSL_LINUX_TEST=1 bash scripts/test/ip-ssl-listener-test.sh
bash scripts/test/ip-ssl-certificate-test.sh
```

Linux integration needs `socat`, `curl`, `openssl` (1.1.1+), and standard coreutils.
The listener tests bind unprivileged random ports on loopback. They need an IPv6
loopback interface, but no public IPv6 address, root privileges, or working
external echo service. Run the same commands on Debian/Ubuntu and AlmaLinux/RHEL;
install test dependencies in a disposable container if they are missing locally.

The tests cover real embedded installer/menu functions, discovery and consent,
unattended overrides, both socket families, actual IP SAN types and matching keys,
failed issuance/installation, rollback, permissions, and the persisted renewal
reload guard. Package installation, ACME orders, and panel/service operations are
mocked at their command boundaries.

To check unchanged upstream acme.sh listener implementations separately:

```bash
curl -fsSLo /tmp/acme-3.1.5.sh https://raw.githubusercontent.com/acmesh-official/acme.sh/3.1.5/acme.sh
bash scripts/test/ip-ssl-acme-listener-test.sh /tmp/acme-3.1.5.sh 28085
curl -fsSLo /tmp/acme-3.1.6.sh https://raw.githubusercontent.com/acmesh-official/acme.sh/3.1.6/acme.sh
bash scripts/test/ip-ssl-acme-listener-test.sh /tmp/acme-3.1.6.sh 28085
```

This invokes only the upstream listener dispatcher, sends independent loopback
HTTP requests with `curl -4` and `curl -6`, prints the sockets with `ss`, then
stops the listeners. It does not issue ACME orders or change an installed client.
Run it in a disposable Linux environment with `iproute2`/`iproute` installed.
Test both `net.ipv6.bindv6only=0` and `1` container configurations where available.

Loopback success does not establish public reachability. Real CA validation,
external port 80 forwarding, provider firewalls, and scheduled cron execution on
a deployed server require separate deployment testing.

After editing `scripts/lib/ip-ssl.sh`, run
`bash scripts/sync-ip-ssl-helpers.sh` to refresh both standalone scripts. CI checks
that the embedded helpers match the canonical source.
