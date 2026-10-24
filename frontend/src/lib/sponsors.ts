import type { Sponsor } from '@/generated/types';

export type SponsorSlot = 'dashboard' | 'page' | 'login';

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
