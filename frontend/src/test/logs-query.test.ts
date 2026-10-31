import { describe, expect, it } from 'vitest';

import { countryBin, countryName } from '@/lib/logs/countryScale';
import {
  EMPTY_LOG_FILTERS,
  formatBucketLabel,
  hasActiveLogFilters,
  logQueryParams,
  splitHighlights,
} from '@/lib/logs/logQuery';
import mapData from '@/pages/logs/worldMapData.json';

describe('countryBin', () => {
  it('gives the peak the top bin and the lowest country the bottom one', () => {
    expect(countryBin(50_000, 1, 50_000)).toBe(5);
    expect(countryBin(1, 1, 50_000)).toBe(1);
  });

  it('spreads a skewed distribution instead of flattening all but the peak', () => {
    const bins = [5, 50, 500, 5000, 100_000].map((count) => countryBin(count, 5, 100_000));
    expect(bins).toEqual([1, 2, 3, 4, 5]);
  });

  it('uses the whole scale for a narrow spread, rather than crowding the top bins', () => {
    expect(countryBin(522, 522, 8153)).toBe(1);
    expect(countryBin(8153, 522, 8153)).toBe(5);
  });

  it('gives a lone country the top bin, however small its count', () => {
    expect(countryBin(1, 1, 1)).toBe(5);
  });
});

describe('countryName', () => {
  it('follows the panel language', () => {
    expect(countryName('DE', 'en-US')).toBe('Germany');
    expect(countryName('DE', 'de-DE')).toBe('Deutschland');
  });

  it('falls back to the code for anything Intl rejects', () => {
    expect(countryName('not-a-region', 'en-US')).toBe('not-a-region');
  });
});

// Every country code in the geoip.dat the panel ships, so none can silently miss the map.
describe('world map data', () => {
  const GEOIP_CODES =
    'AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS XK YE YT ZA ZM ZW'.split(
      ' ',
    );

  it('can place every country the database can return', () => {
    expect(GEOIP_CODES).toHaveLength(250);
    const { shapes, markers } = mapData as {
      shapes: Record<string, string>;
      markers: Record<string, number[]>;
    };
    expect(GEOIP_CODES.filter((code) => !shapes[code] && !markers[code])).toEqual([]);
  });
});

describe('logQueryParams', () => {
  const now = 1_000_000_000;

  it('sends only what is set, trimmed', () => {
    expect(logQueryParams('a.log', EMPTY_LOG_FILTERS, now)).toEqual({ file: 'a.log' });
    expect(
      logQueryParams('a.log', { ...EMPTY_LOG_FILTERS, ip: ' 1.2.3.4 ', user: '   ' }, now),
    ).toEqual({ file: 'a.log', ip: '1.2.3.4' });
  });

  it('ignores custom bounds left over once a preset is selected', () => {
    const stale = { ...EMPTY_LOG_FILTERS, since: 5, until: 6 };
    expect(logQueryParams('a.log', { ...stale, range: '15m' }, now)).toEqual({
      file: 'a.log',
      since: now - 900_000,
    });
    expect(logQueryParams('a.log', { ...stale, range: 'all' }, now)).toEqual({ file: 'a.log' });
    expect(logQueryParams('a.log', { ...stale, range: 'custom' }, now)).toEqual({
      file: 'a.log',
      since: 5,
      until: 6,
    });
  });

  it('maps the level control onto the levels parameter', () => {
    expect(logQueryParams('a.log', { ...EMPTY_LOG_FILTERS, level: 'error' }, now)).toEqual({
      file: 'a.log',
      levels: 'error',
    });
  });
});

describe('hasActiveLogFilters', () => {
  it('counts only filters that change a request', () => {
    expect(hasActiveLogFilters(EMPTY_LOG_FILTERS)).toBe(false);
    expect(hasActiveLogFilters({ ...EMPTY_LOG_FILTERS, host: '   ' })).toBe(false);
    expect(hasActiveLogFilters({ ...EMPTY_LOG_FILTERS, host: 'x' })).toBe(true);
    expect(hasActiveLogFilters({ ...EMPTY_LOG_FILTERS, level: 'info' })).toBe(true);
    expect(hasActiveLogFilters({ ...EMPTY_LOG_FILTERS, range: '1h' })).toBe(true);
  });
});

describe('splitHighlights', () => {
  it('marks every case-insensitive occurrence and keeps the original casing', () => {
    expect(splitHighlights('Foo foo', ['FOO'])).toEqual([
      { text: 'Foo', match: true },
      { text: ' ', match: false },
      { text: 'foo', match: true },
    ]);
  });

  it('treats regex syntax in a term as literal text', () => {
    expect(splitHighlights('a.c abc (x', ['a.c', '(x'])).toEqual([
      { text: 'a.c', match: true },
      { text: ' abc ', match: false },
      { text: '(x', match: true },
    ]);
  });

  it('prefers the longest overlapping term', () => {
    expect(splitHighlights('example.com', ['example', 'example.com'])).toEqual([
      { text: 'example.com', match: true },
    ]);
  });

  it('returns the text whole without a usable term', () => {
    expect(splitHighlights('abc', ['', '  '])).toEqual([{ text: 'abc', match: false }]);
    expect(splitHighlights('', ['a'])).toEqual([{ text: '', match: false }]);
  });
});

describe('formatBucketLabel', () => {
  const at = Date.UTC(2026, 2, 4, 5, 6);
  const HOUR = 3_600_000;
  const DAY = 24 * HOUR;

  it('drops the date while the axis fits within a day', () => {
    expect(formatBucketLabel(at, HOUR, 6 * HOUR, 'en-GB')).not.toMatch(/03/);
  });

  it('adds the date once the axis crosses days', () => {
    expect(formatBucketLabel(at, HOUR, 3 * DAY, 'en-GB')).toMatch(/04\/03/);
  });

  it('drops the time for day-sized buckets, where it would be meaningless', () => {
    expect(formatBucketLabel(at, DAY, 30 * DAY, 'en-GB')).toBe('04/03');
  });
});
