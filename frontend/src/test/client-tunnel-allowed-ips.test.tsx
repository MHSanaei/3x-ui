import { describe, it, expect } from 'vitest';

import {
  parseAllowedIPsList,
  resolveTunnelAllowedIPsByInbound,
} from '@/pages/clients/ClientFormModal';

describe('parseAllowedIPsList', () => {
  it('splits, trims, and drops empty entries', () => {
    expect(parseAllowedIPsList(' 10.0.0.2/32 , 10.0.0.3/32,')).toEqual([
      '10.0.0.2/32',
      '10.0.0.3/32',
    ]);
  });

  it('returns an empty array for a blank string', () => {
    expect(parseAllowedIPsList('')).toEqual([]);
  });
});

describe('resolveTunnelAllowedIPsByInbound', () => {
  // Regression coverage for the bug this whole feature exists to fix: a
  // client attached to both a WireGuard and an AmneziaWG inbound must get
  // each protocol's own address routed to its own inbound id, never the
  // other's -- a single shared field can't represent two different
  // addresses, which is exactly what confused wg's 10.0.0.2/32 with awg's
  // 10.8.1.0/24 subnet in the real production bug report.
  it('maps each protocol field to its own attached inbound id', () => {
    const wireguardIds = new Set([7]);
    const amneziawgIds = new Set([10]);
    const result = resolveTunnelAllowedIPsByInbound(
      [7, 10],
      wireguardIds,
      amneziawgIds,
      ['10.0.0.2/32'],
      ['10.8.1.21/32'],
    );
    expect(result).toEqual({ 7: ['10.0.0.2/32'], 10: ['10.8.1.21/32'] });
  });

  it('omits a protocol entirely when its inbound is not among the attached ids', () => {
    const wireguardIds = new Set([7]);
    const amneziawgIds = new Set([10]);
    const result = resolveTunnelAllowedIPsByInbound(
      [7],
      wireguardIds,
      amneziawgIds,
      ['10.0.0.2/32'],
      ['10.8.1.21/32'],
    );
    expect(result).toEqual({ 7: ['10.0.0.2/32'] });
    expect(result).not.toHaveProperty('10');
  });

  it('returns an empty object when neither protocol is attached', () => {
    const result = resolveTunnelAllowedIPsByInbound([3], new Set([7]), new Set([10]), ['x'], ['y']);
    expect(result).toEqual({});
  });

  it('picks the first matching id when multiple inbounds of the same protocol are attached', () => {
    const wireguardIds = new Set([7, 8]);
    const amneziawgIds = new Set([10]);
    const result = resolveTunnelAllowedIPsByInbound(
      [8, 7, 10],
      wireguardIds,
      amneziawgIds,
      ['10.0.0.2/32'],
      ['10.8.1.21/32'],
    );
    expect(result).toEqual({ 8: ['10.0.0.2/32'], 10: ['10.8.1.21/32'] });
  });
});
