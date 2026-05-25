import { describe, expect, it } from 'vitest';

import { toHeaders, toV2Headers, type HeaderEntry } from '@/lib/xray/headers';
import { XrayCommonClass } from '@/models/inbound';

// Shadow harness: the new pure helpers must agree byte-for-byte with the
// legacy XrayCommonClass static methods. Drift here is a regression.

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

describe('toHeaders parity with XrayCommonClass.toHeaders', () => {
  for (const [label, input] of headerMapCases) {
    it(label, () => {
      expect(toHeaders(input)).toEqual(XrayCommonClass.toHeaders(input));
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

describe('toV2Headers parity (arr=true)', () => {
  for (const [label, input] of entryCases) {
    it(label, () => {
      expect(toV2Headers(input, true)).toEqual(XrayCommonClass.toV2Headers(input, true));
    });
  }
});

describe('toV2Headers parity (arr=false)', () => {
  for (const [label, input] of entryCases) {
    it(label, () => {
      expect(toV2Headers(input, false)).toEqual(XrayCommonClass.toV2Headers(input, false));
    });
  }
});
