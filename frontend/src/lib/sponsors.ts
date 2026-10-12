import type { Sponsor } from '@/generated/types';

export type SponsorSlot = 'dashboard' | 'sidebar' | 'page' | 'login';

const DISMISS_KEY = 'xui.sponsor.dismissed';
export const SPONSOR_DISMISS_MS = 24 * 60 * 60 * 1000;

export function pickLocale(map: Record<string, string> | undefined, lang: string): string {
  if (!map) return '';
  const short = lang.split('-')[0].toLowerCase();
  return map[lang] || map[short] || map.en || '';
}

export function sponsorsForSlot(sponsors: Sponsor[], slot: SponsorSlot): Sponsor[] {
  return sponsors.filter((s) => s.slots.includes(slot));
}

function readDismissed(): Record<string, number> {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(DISMISS_KEY) || '{}');
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, number>) : {};
  } catch {
    return {};
  }
}

export function isSponsorDismissed(id: string, slot: SponsorSlot, now = Date.now()): boolean {
  const at = readDismissed()[`${id}:${slot}`];
  return typeof at === 'number' && now - at < SPONSOR_DISMISS_MS;
}

export function dismissSponsor(id: string, slot: SponsorSlot, now = Date.now()) {
  const next = Object.fromEntries(
    Object.entries(readDismissed()).filter(([, at]) => now - at < SPONSOR_DISMISS_MS),
  );
  next[`${id}:${slot}`] = now;
  try {
    localStorage.setItem(DISMISS_KEY, JSON.stringify(next));
  } catch {}
}

// Mirrors the backend: dashboard/login show one sponsor, the sidebar rotates up to three.
export const SLOT_CAPACITY: Partial<Record<SponsorSlot, number>> = {
  dashboard: 1,
  login: 1,
  sidebar: 3,
};

export interface PlacementStatus {
  count: number;
  capacity?: number;
  takenUntil?: string;
}

// When full, a place frees up once enough bookings end to drop below capacity.
export function placementStatus(sponsors: Sponsor[], slot: SponsorSlot): PlacementStatus {
  const booked = sponsorsForSlot(sponsors, slot);
  const capacity = SLOT_CAPACITY[slot];
  if (!capacity || booked.length < capacity) return { count: booked.length, capacity };
  const ends = booked.map((s) => s.until).sort((a, b) => Date.parse(a) - Date.parse(b));
  return { count: booked.length, capacity, takenUntil: ends[booked.length - capacity] };
}
