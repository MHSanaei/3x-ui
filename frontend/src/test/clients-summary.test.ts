import { describe, it, expect } from 'vitest';

import { computeClientsSummary, pickClientsSummary, sameSpeedMap, sameSummaryInputs } from '@/hooks/useClients';
import type { ClientTraffic, ClientsSummary } from '@/schemas/client';

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

  it('reports a counter alongside every bucket list', () => {
    const stats: Row[] = [
      row({ email: 'online@x', enable: true }),
      row({ email: 'disabled@x', enable: false }),
      row({ email: 'exhausted@x', enable: true, total: 1 * GB, up: 1 * GB }),
      row({ email: 'nearlimit@x', enable: true, total: 10 * GB, up: 9.9 * GB }),
    ];
    const s = computeClientsSummary(stats, new Set(['online@x']), 3 * DAY, 1 * GB);

    // The server caps its lists but never its counters; the live recompute has
    // both, so the summary card reads the same either way.
    expect(s.onlineCount).toBe(s.online.length);
    expect(s.depletedCount).toBe(s.depleted.length);
    expect(s.expiringCount).toBe(s.expiring.length);
    expect(s.deactiveCount).toBe(s.deactive.length);
    expect(s.active + s.depletedCount + s.expiringCount + s.deactiveCount).toBe(s.total);
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

describe('pickClientsSummary', () => {
  const serverSummary: ClientsSummary = {
    total: 67, active: 58,
    onlineCount: 0, depletedCount: 4, expiringCount: 3, deactiveCount: 2,
    online: [], depleted: [], expiring: [], deactive: [],
  };

  it('keeps the server summary when the snapshot is short of the server total (#6102)', () => {
    const shortSnapshot: Row[] = Array.from({ length: 58 }, (_, i) => row({ email: `c${i}@x`, enable: true }));
    const s = pickClientsSummary(serverSummary, shortSnapshot, new Set(), 3 * DAY, 1 * GB);
    expect(s).toEqual(serverSummary);
  });

  it('uses the live recompute when the snapshot covers every client', () => {
    const fullSnapshot: Row[] = Array.from({ length: 67 }, (_, i) => row({ email: `c${i}@x`, enable: true }));
    const s = pickClientsSummary(serverSummary, fullSnapshot, new Set(), 3 * DAY, 1 * GB);
    expect(s.total).toBe(67);
    expect(s.active).toBe(67);
  });

  it('falls back to the server summary before the first WS snapshot arrives', () => {
    const s = pickClientsSummary(serverSummary, [], new Set(), 3 * DAY, 1 * GB);
    expect(s).toEqual(serverSummary);
  });
});

describe('websocket payload identity preservation', () => {
  const speed = (up: number, down: number) => ({ up, down });

  it('treats an unchanged speed map as unchanged', () => {
    const a = { 'a@x': speed(1, 2), 'b@x': speed(3, 4) };
    expect(sameSpeedMap(a, { 'a@x': speed(1, 2), 'b@x': speed(3, 4) })).toBe(true);
    expect(sameSpeedMap(a, { 'a@x': speed(1, 2) })).toBe(false);
    expect(sameSpeedMap(a, { 'a@x': speed(1, 2), 'b@x': speed(3, 5) })).toBe(false);
    expect(sameSpeedMap(a, { 'a@x': speed(1, 2), 'c@x': speed(3, 4) })).toBe(false);
    expect(sameSpeedMap({}, {})).toBe(true);
  });

  it('compares exactly the fields the summary reads, and ignores lastOnline', () => {
    const base: Row[] = [row({ email: 'a@x', up: 1, down: 2, total: 10, expiryTime: 99 })];

    // lastOnline moves for every online client on every push and no counter
    // depends on it, so it must not force a new snapshot.
    const onlyLastOnlineMoved: Row[] = [
      row({ email: 'a@x', up: 1, down: 2, total: 10, expiryTime: 99, lastOnline: 12345 }),
    ];
    expect(sameSummaryInputs(base, onlyLastOnlineMoved)).toBe(true);

    for (const changed of [
      row({ email: 'b@x', up: 1, down: 2, total: 10, expiryTime: 99 }),
      row({ email: 'a@x', up: 2, down: 2, total: 10, expiryTime: 99 }),
      row({ email: 'a@x', up: 1, down: 3, total: 10, expiryTime: 99 }),
      row({ email: 'a@x', up: 1, down: 2, total: 11, expiryTime: 99 }),
      row({ email: 'a@x', up: 1, down: 2, total: 10, expiryTime: 100 }),
      row({ email: 'a@x', up: 1, down: 2, total: 10, expiryTime: 99, enable: false }),
    ]) {
      expect(sameSummaryInputs(base, [changed])).toBe(false);
    }
    expect(sameSummaryInputs(base, [])).toBe(false);
  });
});
