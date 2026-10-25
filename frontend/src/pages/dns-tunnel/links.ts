export interface DnsTunnelProfile {
  name: string;
  engine: 'stormdns' | 'dnstt';
  domain: string;
  key: string;
  method: number;
}

function base64UrlNoPad(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let bin = '';
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// The profile link WhiteDNS imports (and exports): stormdns:// + base64url JSON with the
// "whitedns.profile" schema, as in its WhiteDnsProfileLinks.kt.
export function whiteDnsLink(p: DnsTunnelProfile): string {
  const root = {
    schema: 'whitedns.profile',
    version: 1,
    profile: {
      name: p.name,
      server: { domain: p.domain, encryption_key: p.key, encryption_method: p.method },
    },
  };
  return 'stormdns://' + base64UrlNoPad(JSON.stringify(root));
}

// Resolver lists as WhiteDNS and the StormDNS client read them: one address per line.
export function resolverList(ips: string[]): string {
  return ips.join('\n');
}
