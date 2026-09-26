import { describe, expect, it } from 'vitest';

import { isOutboundProtocol } from '@/schemas/primitives';

// xray-core lowercases a protocol id in LoadWithID before it resolves the
// handler, so every panel reader has to accept the spellings it accepts.
describe('isOutboundProtocol', () => {
  it.each([
    ['canonical', { protocol: 'freedom' }],
    ['capitalised', { protocol: 'Freedom' }],
    ['upper', { protocol: 'FREEDOM' }],
    ['mixed', { protocol: 'fReEdOm' }],
  ])('matches a %s id', (_name, outbound) => {
    expect(isOutboundProtocol(outbound, 'freedom')).toBe(true);
  });

  it.each([
    ['another protocol', { protocol: 'blackhole' }],
    ['another spelling of another protocol', { protocol: 'Blackhole' }],
    ['missing', {}],
    ['empty', { protocol: '' }],
    ['not a string', { protocol: 42 }],
    ['null', null],
    ['undefined', undefined],
  ])('rejects %s', (_name, outbound) => {
    expect(isOutboundProtocol(outbound, 'freedom')).toBe(false);
  });
});
