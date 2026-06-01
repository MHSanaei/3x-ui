import { describe, it, expect } from 'vitest';

import { computeClientsSummary } from '@/hooks/useClients';
import type { ClientTraffic } from '@/schemas/client';

// Parity with web/service/client.go buildClientsSummary: the same client must
// land in the same bucket whether the count comes from the server (list fetch)
// or is recomputed live from the client_stats WS event. A mismatch would make
// the summary card "jump" on refresh.
type Row = ClientTraffic & { email?: string };

const GB = 1024 * 1024 * 1024;
const DAY = 86_400_000;

function row(over: Partial<Row>): Row {
  return { email: 'x', enable: true, up: 0, down: 0, total: 0, expiryTime: 0, ...over } as Row;
}

describe('computeClientsSummary', () => {
  it('buckets each client the way the Go service does', () => {
    const now = Date.now();
    const stats: Row[] = [
      row({ email: 'online@x', enable: true }),
      row({ email: 'offline@x', enable: true }),
      row({ email: 'disabled@x', enable: false }),
      row({ email: 'exhausted@x', enable: true, total: 1 * GB, up: 1 * GB }),
      row({ email: 'expired@x', enable: true, expiryTime: now - DAY }),
      row({ email: 'nearexpiry@x', enable: true, expiryTime: now + DAY }),
      row({ email: 'nearlimit@x', enable: true, total: 10 * GB, up: 9.9 * GB }),
    ];
    const online = new Set(['online@x', 'disabled@x']); // disabled-but-online must NOT count as online
    const expireDiffMs = 3 * DAY;
    const trafficDiffBytes = 1 * GB;

    const s = computeClientsSummary(stats, online, expireDiffMs, trafficDiffBytes);

    expect(s.total).toBe(7);
    expect(s.online).toEqual(['online@x']);
    expect(s.depleted.sort()).toEqual(['exhausted@x', 'expired@x']);
    expect(s.deactive).toEqual(['disabled@x']);
    expect(s.expiring.sort()).toEqual(['nearexpiry@x', 'nearlimit@x']);
    expect(s.active).toBe(2); // online@x + offline@x
  });

  it('depleted wins over disabled and over online', () => {
    const stats: Row[] = [
      row({ email: 'a@x', enable: false, total: 1 * GB, up: 2 * GB }),
    ];
    const s = computeClientsSummary(stats, new Set(['a@x']), 0, 0);
    expect(s.depleted).toEqual(['a@x']);
    expect(s.deactive).toEqual([]);
    expect(s.online).toEqual([]); // disabled is never online
  });

  it('unlimited + no expiry is active', () => {
    const stats: Row[] = [row({ email: 'a@x', enable: true, total: 0, expiryTime: 0 })];
    const s = computeClientsSummary(stats, new Set(), 3 * DAY, 1 * GB);
    expect(s.active).toBe(1);
    expect(s.expiring).toEqual([]);
    expect(s.depleted).toEqual([]);
  });
});
