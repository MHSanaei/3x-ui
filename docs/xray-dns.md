# Xray DNS presets and leak notes

The panel DNS preset menu supports plain DNS, DoH, and DoQ server entries accepted by Xray's built-in DNS config. These notes follow Xray's DNS server address forms documented in <https://xtls.github.io/en/config/dns.html>.

- Plain DNS uses UDP/53 and follows routing unless a local-mode scheme is used. It is easy to observe on the server network path.
- DoH uses `https://.../dns-query` and goes through Xray routing.
- DoQ uses `quic+local://...` in Xray. Local mode bypasses Xray routing and connects directly through Freedom, which avoids DNS routing loops but can reveal resolver traffic from the server IP.
- Xray DNS does not have a direct DoT URL scheme. Use DoH/DoQ, or wrap DNS-over-TCP/TLS outside this DNS config if DoT is required.

Leak-prone settings:

- `localhost` uses the host resolver and is outside Xray control.
- `tcp+local://`, `https+local://`, and `quic+local://` bypass Xray routing by design.
- Domain resolver names can need system DNS in local mode unless pinned in `hosts`.
- `disableFallback: false` or `enableParallelQuery: true` can query fallback servers and reveal domains to more than one provider.
- `clientIp` sends EDNS Client Subnet data upstream.
- Plain IP/UDP DNS presets are not encrypted.

For privacy-sensitive setups, prefer DoH through routed outbounds, add `hosts` pins for resolver hostnames, set `disableFallback` or `skipFallback` where fallback is not wanted, and keep `clientIp` empty.
