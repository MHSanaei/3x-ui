# 3x-ui-20261107 upgrade and rollback

This dated branch preserves the original `main` branch and adds IPv6-aware
DNS leak protection and related regression tests. It is intended as a
reviewable source version; it does not replace the stable or `dev-latest`
release channels.

## What changed from the original version

- The host DNS guard uses an nftables `inet` table and explicitly rejects
  external IPv4 and IPv6 plaintext DNS on TCP/UDP port 53.
- Both locally generated traffic (`output`) and forwarded/container traffic
  (`forward`) are covered while loopback DNS remains available.
- Guard installation verifies the resolver, listener, nftables rules, and a
  real DNS lookup. An incomplete activation is rolled back automatically.
- Default subscription/Xray tests assert dual-stack DNS resolution and a
  family-neutral final TCP/UDP proxy rule.
- The documentation explains the boundary of non-TUN WebRTC protection:
  traffic that reaches Xray is covered, but traffic sent directly by the
  browser outside the proxy cannot be intercepted by the server.

## Before updating

Run these commands separately as root. Keep the backups until the new version
has been verified on a network that actually provides public IPv6.

```bash
sudo systemctl stop x-ui
sudo cp -a /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.before-3x-ui-20261107
sudo cp -a /usr/local/x-ui /usr/local/x-ui.before-3x-ui-20261107
sudo cp -a /usr/bin/x-ui /usr/bin/x-ui.before-3x-ui-20261107
sudo systemctl start x-ui
```

If certificates or environment overrides are stored outside those paths,
back them up as well (for example `/etc/default/x-ui` on Debian/Ubuntu).

## Build and update from this branch

The dated branch is deliberately separate from the normal release updater.
Build it in a temporary directory, then replace only the application binary
after making the backups above.

```bash
sudo apt-get update
sudo apt-get install -y git curl ca-certificates build-essential pkg-config libsqlite3-dev

git clone --branch 3x-ui-20261107 --depth 1 \
  https://github.com/jrangus/3x-ui.git /tmp/3x-ui-20261107
cd /tmp/3x-ui-20261107

# Use the Go and Node versions declared by go.mod and frontend/package.json.
cd frontend
npm ci
npm run build
cd ..
go test ./internal/sub ./internal/xray/dnsconf
go build -trimpath -o /tmp/x-ui-3x-ui-20261107 .

sudo systemctl stop x-ui
sudo install -m 755 /tmp/x-ui-3x-ui-20261107 /usr/local/x-ui/x-ui
sudo install -m 755 x-ui.sh /usr/bin/x-ui
sudo systemctl start x-ui
```

The database is retained. Do not delete `/etc/x-ui`, and do not run a clean
installation over the existing server.

## Enable and verify the IPv6-aware DNS guard

Open the management menu and select **Network Privacy & DNS Guard**, or invoke
the corresponding menu action interactively:

```bash
sudo x-ui
```

Then verify the service and rules:

```bash
sudo systemctl is-active x-ui x-ui-dns-guard.service
sudo nft -nn list table inet xui_dns_guard
ip -6 route show default
dig A example.com
dig AAAA example.com
```

Test WebRTC and DNS from both desktop and mobile clients on a network with
working public IPv6. A test performed only on IPv4 cannot prove IPv6 leak
protection.

## Roll back to the saved original version

If the panel or proxy fails validation, restore the exact files saved before
the update:

```bash
sudo systemctl stop x-ui
sudo test ! -e /usr/local/x-ui.failed-3x-ui-20261107
sudo mv /usr/local/x-ui /usr/local/x-ui.failed-3x-ui-20261107
sudo cp -a /usr/local/x-ui.before-3x-ui-20261107 /usr/local/x-ui
sudo cp -a /usr/bin/x-ui.before-3x-ui-20261107 /usr/bin/x-ui
sudo cp -a /etc/x-ui/x-ui.db.before-3x-ui-20261107 /etc/x-ui/x-ui.db
sudo systemctl start x-ui
```

If the DNS guard itself is the only problem, disable it without rolling back
the whole panel:

```bash
sudo x-ui disable-dns-guard
```

After rollback, verify:

```bash
sudo systemctl --no-pager --full status x-ui
/usr/local/x-ui/x-ui -v
```

The failed installation is retained at
`/usr/local/x-ui.failed-3x-ui-20261107` for diagnosis. Remove it only after the
restored version has been confirmed healthy.
