import { describe, expect, it } from 'vitest';

import {
  defaultSelection,
  nearbyIps,
  parseTargets,
  resultsCsv,
  scanRanges,
} from '@/pages/clean-ip/scan';

describe('parseTargets', () => {
  it('takes single IPs as they are, samples ranges, and drops junk and repeats', () => {
    const got = parseTargets(
      '104.16.1.1\n104.16.1.1, 1.2.3\n  162.159.10.0/30 # note\nnot-an-ip 300.1.1.1',
      8,
    );
    expect(got.slice(0, 1)).toEqual(['104.16.1.1']);
    expect(got.slice(1).sort()).toEqual(['162.159.10.1', '162.159.10.2']);
  });
});

describe('nearbyIps', () => {
  it('picks untested addresses in the /24 of each working IP', () => {
    const tested = new Set(['104.16.5.10', '104.16.5.11', '172.66.1.200']);
    const got = nearbyIps(['104.16.5.10', '172.66.1.200'], tested, 4);
    expect(got).toHaveLength(8);
    expect(got.filter((ip) => ip.startsWith('104.16.5.'))).toHaveLength(4);
    expect(got.filter((ip) => ip.startsWith('172.66.1.'))).toHaveLength(4);
    for (const ip of got) {
      expect(tested.has(ip)).toBe(false);
      const last = Number(ip.split('.')[3]);
      expect(last >= 1 && last <= 254).toBe(true);
    }
    expect(new Set(got).size).toBe(8);
  });
});

describe('resultsCsv', () => {
  it('writes a header and one line per IP with its datacenter place', () => {
    expect(
      resultsCsv(
        [
          { ip: '104.16.1.1', ms: 120, colo: 'FRA' },
          { ip: '104.16.1.2', ms: 300, colo: '' },
        ],
        'en-US',
      ),
    ).toBe('ip,ms,colo,city,country\n104.16.1.1,120,FRA,Frankfurt,DE\n104.16.1.2,300,,,\n');
  });
});

describe('range selection', () => {
  const cdn = {
    id: 'cloudflare',
    name: 'Cloudflare',
    groups: [
      { id: 'official', on: true, ranges: ['104.16.0.0/13', '172.64.0.0/13'] },
      { id: 'as13335', on: false, ranges: ['5.83.149.0/24'] },
    ],
  };

  it('starts from the groups that are on, so opt-in AS ranges are not scanned', () => {
    expect(defaultSelection(cdn)).toEqual(['104.16.0.0/13', '172.64.0.0/13']);
  });

  it('scans the ticked ranges plus valid extra ones, once each', () => {
    expect(
      scanRanges(['104.16.0.0/13'], '104.16.0.0/13\n198.41.128.0/17, junk 1.2.3.4 8.8.8.0/33'),
    ).toEqual(['104.16.0.0/13', '198.41.128.0/17']);
  });
});
