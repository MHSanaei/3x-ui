import type { CleanIpReport } from './FragmentStep';

export interface IpRow {
  ip: string;
  ms: number;
  test: 'upload' | 'quick' | 'failed';
  colo: string;
}

// The one-day command may run on another ISP than the admin's browser (the "via" command
// is for ISPs that block the panel's domain): its result belongs to that ISP.
export function reportFromOtherIsp(report: CleanIpReport | undefined, browserAsn: number): boolean {
  return !!report && !!report.network.asn && report.network.asn !== browserAsn;
}

export function saveIsp(opts: {
  browserIsp: string;
  browserAsn: number;
  report: CleanIpReport | undefined;
  typedIsp: string | null;
}): string {
  const { browserIsp, browserAsn, report, typedIsp } = opts;
  if (report && reportFromOtherIsp(report, browserAsn))
    return (report.network.isp || typedIsp || '').trim();
  return (typedIsp ?? browserIsp).trim();
}

// Upload-tested IPs first, then quick-scan hits; a hit the command tried without an upload
// getting through is "failed". Hits seen only from another ISP than the report's are dropped.
export function resultRows(
  report: CleanIpReport | undefined,
  hits: { ip: string; ms: number }[],
  sentIps: string[],
  browserAsn: number,
): IpRow[] {
  const out = new Map<string, IpRow>();
  for (const h of report?.ips ?? [])
    out.set(h.ip, { ip: h.ip, ms: h.ms, test: 'upload', colo: h.colo });
  const sent = new Set(report ? sentIps : []);
  const otherIsp = reportFromOtherIsp(report, browserAsn);
  for (const h of hits) {
    if (out.has(h.ip) || (otherIsp && !sent.has(h.ip))) continue;
    out.set(h.ip, { ...h, test: sent.has(h.ip) ? 'failed' : 'quick', colo: '' });
  }
  const rank = { upload: 0, quick: 1, failed: 2 };
  return [...out.values()].sort((a, b) => rank[a.test] - rank[b.test] || a.ms - b.ms);
}
