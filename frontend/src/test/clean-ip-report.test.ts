import { describe, expect, it } from 'vitest';

import { resultRows, saveIsp } from '@/pages/clean-ip/report';
import type { CleanIpReport } from '@/pages/clean-ip/FragmentStep';

const SHT = { asn: 31549, isp: 'SHT' };
const MCI = { asn: 197207, isp: 'MCI' };

function reportFrom(network: { asn: number; isp: string }, ips: string[]): CleanIpReport {
  return {
    at: 1,
    network: { ip: '5.160.10.20', org: '', ...network },
    tested: ips.length,
    size: 5000,
    ips: ips.map((ip, i) => ({ ip, ms: 100 + i, colo: 'FRA' })),
    fragments: [],
  };
}

// The "via" command exists for an ISP that blocks the panel's domain, so the phone running it
// is often on another ISP than the admin's browser: its IPs must not be saved under the browser's.
describe('a full-test report from another ISP', () => {
  const report = reportFrom(MCI, ['104.16.1.1']);
  const hits = [
    { ip: '104.16.1.1', ms: 50 },
    { ip: '104.16.2.2', ms: 60 },
    { ip: '104.16.3.3', ms: 70 },
  ];

  it('is saved under the ISP it came from, not the browser one', () => {
    expect(saveIsp({ browserIsp: 'SHT', browserAsn: SHT.asn, report, typedIsp: null })).toBe('MCI');
    expect(
      saveIsp({ browserIsp: 'SHT', browserAsn: SHT.asn, report: undefined, typedIsp: null }),
    ).toBe('SHT');
  });

  it("leaves out the browser's quick-scan hits, which were only seen from the browser ISP", () => {
    const rows = resultRows(report, hits, ['104.16.1.1', '104.16.2.2'], SHT.asn);
    expect(rows.map((r) => [r.ip, r.test])).toEqual([
      ['104.16.1.1', 'upload'],
      ['104.16.2.2', 'failed'],
    ]);
  });

  it('keeps every quick-scan hit when the report is from the same ISP', () => {
    const same = reportFrom(SHT, ['104.16.1.1']);
    const rows = resultRows(same, hits, ['104.16.1.1', '104.16.2.2'], SHT.asn);
    expect(rows.map((r) => [r.ip, r.test])).toEqual([
      ['104.16.1.1', 'upload'],
      ['104.16.3.3', 'quick'],
      ['104.16.2.2', 'failed'],
    ]);
  });

  it('needs a typed name when the reporting ISP has no short name', () => {
    const unnamed = reportFrom({ asn: 64500, isp: '' }, ['104.16.1.1']);
    expect(
      saveIsp({ browserIsp: 'SHT', browserAsn: SHT.asn, report: unnamed, typedIsp: null }),
    ).toBe('');
    expect(
      saveIsp({ browserIsp: 'SHT', browserAsn: SHT.asn, report: unnamed, typedIsp: ' Zap ' }),
    ).toBe('Zap');
  });
});
