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
      server: {
        domain: p.domain,
        encryption_key: p.key,
        encryption_method: p.method,
      },
    },
  };
  return 'stormdns://' + base64UrlNoPad(JSON.stringify(root));
}

// Resolver lists as WhiteDNS and the StormDNS client read them: one address per line.
export function resolverList(ips: string[]): string {
  return ips.join('\n');
}

export interface ResolverResult {
  ip: string;
  status: string;
  ms: number;
  name: string;
  maxName: number;
}

// The longest DNS name; a resolver that carries names this long does not cap the upload MTU.
export const NAME_MAX = 253;

export function isLowMtu(r: ResolverResult): boolean {
  return r.maxName > 0 && r.maxName < NAME_MAX;
}

// StormDNS runs every resolver at the smallest MTU among them (internal/client/mtu.go), so
// low-MTU resolvers stay out of a shared list unless nothing else works.
export function resolverShare(results: ResolverResult[]): {
  ips: string[];
  low: boolean;
  leftOut: number;
} {
  const ok = results.filter((r) => r.status === 'ok');
  const full = ok.filter((r) => !isLowMtu(r)).map((r) => r.ip);
  if (full.length) return { ips: full, low: false, leftOut: ok.length - full.length };
  return { ips: ok.map((r) => r.ip), low: ok.length > 0, leftOut: 0 };
}

// A server read from this machine replaces the listed one with its domain (the key may have
// been regenerated) but keeps the admin's name for it.
export function mergeDetected(
  listed: DnsTunnelProfile[],
  detected: DnsTunnelProfile[],
): DnsTunnelProfile[] {
  const byDomain = new Map(detected.map((d) => [d.domain, d]));
  const merged = listed.map((p) => {
    const d = p.engine === 'stormdns' ? byDomain.get(p.domain) : undefined;
    if (!d) return p;
    byDomain.delete(p.domain);
    return { ...d, name: p.name };
  });
  return [...merged, ...byDomain.values()];
}
