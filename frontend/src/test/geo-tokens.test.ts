import { describe, expect, it } from 'vitest';

import {
  formatTokens,
  mergeSelection,
  parseTokens,
  selectionFromValue,
  tokenFor,
} from '@/lib/xray/geoTokens';

const siteKnown = new Set(['geosite:google', 'geosite:google@ads', 'geosite:cn', 'ext:my_rules.dat:corp']);
const ipKnown = new Set(['geoip:cn', 'geoip:private', 'ext:my_ips.dat:office']);

describe('parseTokens / formatTokens', () => {
  const cases: Array<[string, string, string[]]> = [
    ['empty value', '', []],
    ['single token', 'geosite:google', ['geosite:google']],
    ['trims and drops blanks', ' geosite:google , , google.com ,', ['geosite:google', 'google.com']],
    ['keeps negation', '!geoip:cn, 10.0.0.0/8', ['!geoip:cn', '10.0.0.0/8']],
  ];

  it.each(cases)('%s', (_name, value, expected) => {
    expect(parseTokens(value)).toEqual(expected);
  });

  it('joins with a comma and a space', () => {
    expect(formatTokens(['geosite:google', 'google.com'])).toBe('geosite:google, google.com');
    expect(formatTokens([])).toBe('');
  });
});

describe('tokenFor', () => {
  const cases: Array<[string, string, string, 'site' | 'ip', string]> = [
    ['default site database uses the geosite shorthand', 'geosite.dat', 'google', 'site', 'geosite:google'],
    ['default ip database uses the geoip shorthand', 'geoip.dat', 'cn', 'ip', 'geoip:cn'],
    ['custom site database falls back to ext', 'my_rules.dat', 'corp', 'site', 'ext:my_rules.dat:corp'],
    ['custom ip database falls back to ext', 'my_ips.dat', 'office', 'ip', 'ext:my_ips.dat:office'],
    ['ip kind on the site database is not shorthand', 'geosite.dat', 'cn', 'ip', 'ext:geosite.dat:cn'],
    ['site kind on the ip database is not shorthand', 'geoip.dat', 'cn', 'site', 'ext:geoip.dat:cn'],
  ];

  it.each(cases)('%s', (_name, file, code, kind, expected) => {
    expect(tokenFor(file, code, kind)).toBe(expected);
  });
});

describe('selectionFromValue', () => {
  const cases: Array<[string, string, ReadonlySet<string>, string[]]> = [
    ['empty value selects nothing', '', siteKnown, []],
    ['plain values are not selectable', 'google.com, keyword:ads', siteKnown, []],
    ['picks known tokens only', 'google.com, geosite:google, geosite:blabla', siteKnown, ['geosite:google']],
    [
      'keeps the value order',
      'geosite:cn, google.com, geosite:google',
      siteKnown,
      ['geosite:cn', 'geosite:google'],
    ],
    ['drops duplicates', 'geosite:google, geosite:google', siteKnown, ['geosite:google']],
    ['attributes are distinct tokens', 'geosite:google@ads', siteKnown, ['geosite:google@ads']],
    ['ext tokens are selectable', 'ext:my_rules.dat:corp, ext:other.dat:x', siteKnown, ['ext:my_rules.dat:corp']],
    ['negated ip tokens stay unselected', '!geoip:cn, geoip:private', ipKnown, ['geoip:private']],
  ];

  it.each(cases)('%s', (_name, value, known, expected) => {
    expect(selectionFromValue(value, known)).toEqual(expected);
  });
});

describe('mergeSelection', () => {
  const cases: Array<[string, string, string[], ReadonlySet<string>, string]> = [
    ['adds to an empty field', '', ['geosite:google'], siteKnown, 'geosite:google'],
    [
      'adds after a plain domain',
      'google.com',
      ['geosite:cn'],
      siteKnown,
      'google.com, geosite:cn',
    ],
    [
      'keeps plain and unknown tokens when a category is unchecked',
      'google.com, geosite:google, geosite:blabla',
      [],
      siteKnown,
      'google.com, geosite:blabla',
    ],
    [
      'unchecking one known token leaves the other known token',
      'geosite:google, geosite:cn',
      ['geosite:cn'],
      siteKnown,
      'geosite:cn',
    ],
    [
      'preserves the original order of surviving tokens',
      'geosite:cn, google.com, geosite:google',
      ['geosite:google', 'geosite:cn'],
      siteKnown,
      'geosite:cn, google.com, geosite:google',
    ],
    [
      'appends new selections in selection order',
      'google.com',
      ['geosite:cn', 'geosite:google'],
      siteKnown,
      'google.com, geosite:cn, geosite:google',
    ],
    [
      'never duplicates an already present token',
      'geosite:google, google.com',
      ['geosite:google'],
      siteKnown,
      'geosite:google, google.com',
    ],
    [
      'collapses duplicates already in the field',
      'google.com, google.com, geosite:google',
      ['geosite:google'],
      siteKnown,
      'google.com, geosite:google',
    ],
    [
      'handles ext tokens like shorthand ones',
      'ext:my_rules.dat:corp, google.com',
      [],
      siteKnown,
      'google.com',
    ],
    [
      'adds an ext token from a custom database',
      '10.0.0.0/8',
      ['ext:my_ips.dat:office'],
      ipKnown,
      '10.0.0.0/8, ext:my_ips.dat:office',
    ],
    [
      'leaves a negated ip token untouched while dropping a plain one',
      '!geoip:cn, geoip:private, 192.168.0.0/16',
      [],
      ipKnown,
      '!geoip:cn, 192.168.0.0/16',
    ],
    [
      'adds a geoip token next to an existing negation',
      '!geoip:cn',
      ['geoip:private'],
      ipKnown,
      '!geoip:cn, geoip:private',
    ],
    [
      'ignores whitespace around field tokens',
      '  google.com ,  geosite:google  ',
      ['geosite:google'],
      siteKnown,
      'google.com, geosite:google',
    ],
    ['clearing every known token can empty the field', 'geosite:google', [], siteKnown, ''],
  ];

  it.each(cases)('%s', (_name, value, selected, known, expected) => {
    expect(mergeSelection(value, selected, known)).toBe(expected);
  });

  it('round-trips with selectionFromValue', () => {
    const value = mergeSelection('google.com, geosite:blabla', ['geosite:google', 'geosite:cn'], siteKnown);
    expect(value).toBe('google.com, geosite:blabla, geosite:google, geosite:cn');
    expect(selectionFromValue(value, siteKnown)).toEqual(['geosite:google', 'geosite:cn']);
  });
});

describe('token matching tolerates the spellings Xray accepts', () => {
  const cases: Array<[string, string, string[], string]> = [
    [
      'an uppercase token is recognised instead of duplicated',
      'GEOSITE:GOOGLE',
      ['geosite:google'],
      'GEOSITE:GOOGLE',
    ],
    [
      'the long ext form of a default database is the same token as its shorthand',
      'ext:geosite.dat:google',
      ['geosite:google'],
      'ext:geosite.dat:google',
    ],
    [
      'clearing a category written in its long form removes it',
      'google.com, ext:geosite.dat:google',
      [],
      'google.com',
    ],
    [
      'clearing a category written in uppercase removes it',
      'GEOSITE:GOOGLE, google.com',
      [],
      'google.com',
    ],
    [
      'a token from a database that was never opened survives untouched',
      'ext:other.dat:x, geosite:google',
      ['geosite:cn'],
      'ext:other.dat:x, geosite:cn',
    ],
    ['a value of separators alone collapses to empty', ',,, ,', [], ''],
  ];

  it.each(cases)('%s', (_name, value, selected, expected) => {
    expect(mergeSelection(value, selected, siteKnown)).toBe(expected);
  });

  it('seeds the selection from tokens written in another spelling', () => {
    expect(selectionFromValue('GEOSITE:GOOGLE, ext:geosite.dat:cn', siteKnown)).toEqual([
      'GEOSITE:GOOGLE',
      'ext:geosite.dat:cn',
    ]);
  });
});
