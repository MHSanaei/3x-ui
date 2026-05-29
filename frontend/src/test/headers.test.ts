import { describe, expect, it } from 'vitest';

import { getHeaderValue, toHeaders, toV2Headers, type HeaderEntry } from '@/lib/xray/headers';

// Pure-function tests for the header helpers. Snapshots were locked at
// the close of the legacy class migration — at that point toHeaders and
// toV2Headers were verified byte-equal to the legacy XrayCommonClass
// static methods. Drift past this baseline is a regression.

const headerMapCases: Array<[string, unknown]> = [
  ['null', null],
  ['undefined', undefined],
  ['primitive', 'not-an-object'],
  ['empty', {}],
  ['single string', { Host: 'example.test' }],
  ['single array', { Host: ['a.example.test'] }],
  ['multi array', { Accept: ['text/html', 'application/json'] }],
  ['mixed', { Host: 'a.example.test', 'X-Trace': ['1', '2'] }],
];

describe('toHeaders', () => {
  for (const [label, input] of headerMapCases) {
    it(label, () => {
      expect(toHeaders(input)).toMatchSnapshot();
    });
  }
});

const entryCases: Array<[string, HeaderEntry[]]> = [
  ['empty', []],
  ['single', [{ name: 'Host', value: 'example.test' }]],
  ['duplicate name', [
    { name: 'Accept', value: 'text/html' },
    { name: 'Accept', value: 'application/json' },
  ]],
  ['empty name skipped', [
    { name: '', value: 'ignored' },
    { name: 'X-Real', value: 'kept' },
  ]],
  ['empty value skipped', [
    { name: 'X-Empty', value: '' },
    { name: 'X-Real', value: 'kept' },
  ]],
];

describe('toV2Headers (arr=true)', () => {
  for (const [label, input] of entryCases) {
    it(label, () => {
      expect(toV2Headers(input, true)).toMatchSnapshot();
    });
  }
});

describe('toV2Headers (arr=false)', () => {
  for (const [label, input] of entryCases) {
    it(label, () => {
      expect(toV2Headers(input, false)).toMatchSnapshot();
    });
  }
});

describe('getHeaderValue lookups', () => {
  it('returns empty string for missing map', () => {
    expect(getHeaderValue(undefined, 'host')).toBe('');
    expect(getHeaderValue(null, 'host')).toBe('');
    expect(getHeaderValue({}, 'host')).toBe('');
  });

  it('finds a string-valued header case-insensitively', () => {
    expect(getHeaderValue({ Host: 'example.test' }, 'host')).toBe('example.test');
    expect(getHeaderValue({ host: 'example.test' }, 'HOST')).toBe('example.test');
  });

  it('returns first value when the header is an array', () => {
    expect(getHeaderValue({ Accept: ['text/html', 'application/json'] }, 'accept')).toBe('text/html');
  });

  it('returns empty string when the header has empty array', () => {
    expect(getHeaderValue({ Host: [] }, 'host')).toBe('');
  });

  it('returns empty string for missing header name', () => {
    expect(getHeaderValue({ Host: 'x' }, 'origin')).toBe('');
  });
});
