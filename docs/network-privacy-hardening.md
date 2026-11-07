# Network performance and leak boundaries

## BBR / BBRv3

The `x-ui` management script enables the BBR implementation supplied by the
running Linux kernel and pairs it with the `fq` pacing qdisc. Linux registers
all BBR generations under the congestion-control name `bbr`; there is no
stable sysctl that identifies the implementation as v1, v2, or v3.

For that reason, x-ui does not download or install an unsigned third-party
kernel. To use BBRv3, first install a trusted distribution or cloud-provider
kernel that explicitly documents BBRv3 support, reboot into that kernel, then
select **Enable kernel BBR** in the x-ui menu. The script verifies that `bbr`
is actually available before changing the active congestion control, persists
module loading, and records the previous qdisc and congestion control so the
change can be reversed.

Use **Show BBR status** to display the running kernel, available algorithms,
active algorithm, qdisc, and module state.

## DNS leakage

The default generated JSON subscription uses DNS over HTTPS to an IP endpoint
instead of plain UDP DNS. Using an IP endpoint also avoids a plaintext DNS
bootstrap lookup for the encrypted resolver hostname. This protects DNS
requests made by clients that actually use the generated profile and send
their DNS through the tunnel.

Server firewall rules cannot safely guarantee this for every client: browsers
and applications may bypass a normal HTTP/SOCKS proxy, and blocking all
outbound port 53 on the server can break the host resolver, containers, ACME,
or custom deployments. For leak-resistant clients, use a TUN/full-route mode,
route DNS through the tunnel, and enable the client's kill switch.

The management menu now includes **Network Privacy & DNS Guard**. Its DNS
egress guard uses an `inet` nftables table with explicit IPv4 and IPv6 rules to
block external TCP and UDP port 53 for both host-originated and
forwarded/container traffic. Separate counters make both address families
visible when inspecting the ruleset. It enables only
after every resolver in `/etc/resolv.conf` is a loopback address, a port-53
listener is bound to each address, and test DNS queries succeed both before and
after the blocking rules are loaded. If the local resolver depended on
external plaintext port 53, the post-enable query fails and the change is
rolled back automatically. This
fail-closed preflight is intentional: it prevents the panel from cutting off
DNS on a remotely managed VPS. The status command also verifies the systemd
service and the expected output/forward reject rules instead of treating an
empty table as enabled. Disable removes the table, service, and generated rules
file.

The same functions are available non-interactively for automation:

```bash
x-ui privacy-status
x-ui enable-dns-guard
x-ui disable-dns-guard
```

The menu also opens or closes explicit TCP/UDP ports through an already-active
UFW or firewalld installation. It never starts a firewall with guessed rules;
doing so remotely could lock out SSH or the panel.

## WebRTC leakage

WebRTC address leakage happens on the client when the browser sends STUN/ICE
traffic outside the proxy. A remote x-ui server never receives that direct
traffic and therefore cannot block it. Some browser/client proxy combinations
route or suppress WebRTC successfully without an explicit TUN mode, but that
behavior is client-, browser-, and network-dependent. Reliable mitigation and
verification must therefore be applied on the client:

1. verify the active browser/client combination with IPv4 and IPv6; use
   TUN/full-route mode when a stronger routing guarantee is required;
2. enable a kill switch that rejects traffic outside the tunnel;
3. configure the browser to use the default route for WebRTC or disable WebRTC
   where policy permits;
4. repeat the leak test after browser, client, or network changes.

Blocking only the common STUN ports on the server is not presented as a fix:
STUN can use other ports, including 443, and such a rule can break QUIC and
legitimate UDP applications while leaving direct browser leaks untouched.
The default JSON subscription's final `tcp,udp -> proxy` rule is address-family
neutral, so IPv6 DNS and WebRTC UDP that actually reaches Xray follows the same
proxy path as IPv4. It cannot capture traffic that the client or browser sends
outside Xray; test on a network that really provides public IPv6, since testing
on an IPv4-only network cannot demonstrate IPv6 leak protection.

