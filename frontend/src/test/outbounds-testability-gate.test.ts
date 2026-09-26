import { describe, expect, it } from 'vitest';

import { isUntestable } from '@/pages/xray/outbounds/outbounds-tab-helpers';
import type { OutboundRow } from '@/pages/xray/outbounds/outbounds-tab-types';

// The row's Test button is disabled by this gate alone: a spelling the core
// resolves but the gate misses reports the panel host's own reachability as a tunnel.
const row = (protocol: string, tag = 'probe'): OutboundRow => ({ key: 0, tag, protocol });

describe('isUntestable', () => {
  it.each(['Freedom', 'FREEDOM', 'fReEdOm'])('disables a %s row', (protocol) => {
    expect(isUntestable(row(protocol))).toBe(true);
  });

  it.each(['DNS', 'Dns'])('disables a %s row', (protocol) => {
    expect(isUntestable(row(protocol))).toBe(true);
  });

  it.each(['Blackhole', 'BLACKHOLE', 'Loopback'])('disables a %s row', (protocol) => {
    expect(isUntestable(row(protocol))).toBe(true);
  });

  it.each(['vless', 'VMess', 'Trojan', 'Socks'])('leaves a %s row testable', (protocol) => {
    expect(isUntestable(row(protocol))).toBe(false);
  });
});
