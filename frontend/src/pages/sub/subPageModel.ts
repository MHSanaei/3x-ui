const DAY_MS = 86_400_000;

export type SubStatus = 'active' | 'unlimited' | 'expired' | 'depleted' | 'disabled';

export interface SubUsage {
  enabled: boolean;
  usedByte: number;
  totalByte: number;
  expireMs: number;
}

export function resolveSubStatus(sub: SubUsage, now: number): SubStatus {
  if (!sub.enabled) return 'disabled';
  if (sub.expireMs > 0 && now >= sub.expireMs) return 'expired';
  if (sub.totalByte > 0 && sub.usedByte >= sub.totalByte) return 'depleted';
  if (sub.totalByte <= 0 && sub.expireMs === 0) return 'unlimited';
  return 'active';
}

export function daysUntil(expireMs: number, now: number): number | null {
  if (expireMs <= 0) return null;
  return Math.max(0, Math.ceil((expireMs - now) / DAY_MS));
}

export function usagePercent(usedByte: number, totalByte: number): number {
  if (totalByte <= 0) return 0;
  const pct = (usedByte / totalByte) * 100;
  return Number.isFinite(pct) ? Math.min(100, Math.max(0, pct)) : 0;
}

export type AppPlatform = 'android' | 'ios';

export function detectPlatform(userAgent: string): AppPlatform {
  // iPadOS sends a Macintosh UA, and App Store clients also run on Apple-silicon Macs.
  if (/iphone|ipad|ipod|macintosh/i.test(userAgent)) return 'ios';
  return 'android';
}

export interface SubApp {
  name: string;
  url: string;
}

export interface SubAppSource {
  subUrl: string;
  sId: string;
  subTitle: string;
}

export function buildSubApps({
  subUrl,
  sId,
  subTitle,
}: SubAppSource): Record<AppPlatform, SubApp[]> {
  const encSub = encodeURIComponent(subUrl);
  const profileName = encodeURIComponent(subTitle || sId);

  const v2box = {
    name: 'V2Box',
    url: `v2box://install-sub?url=${encSub}&name=${encodeURIComponent(sId)}`,
  };
  const singBox = {
    name: 'Sing-box',
    url: `sing-box://import-remote-profile?url=${encSub}#${profileName}`,
  };
  const v2raytun = { name: 'V2RayTun', url: `v2raytun://import/${subUrl}` };
  const happ = { name: 'Happ', url: `happ://add/${subUrl}` };
  const incy = { name: 'Incy', url: `incy://add/${subUrl}` };
  const rocketSource = `${subUrl}${subUrl.includes('?') ? '&' : '?'}flag=shadowrocket`;
  const rocketRemark = encodeURIComponent(subTitle || sId || 'Subscription');

  return {
    android: [
      v2box,
      { name: 'V2RayNG', url: `v2rayng://install-config?url=${encSub}` },
      singBox,
      v2raytun,
      happ,
      incy,
    ],
    ios: [
      {
        name: 'Shadowrocket',
        url: `shadowrocket://add/sub://${btoa(rocketSource)}?remark=${rocketRemark}`,
      },
      v2box,
      { name: 'Streisand', url: `streisand://import/${encSub}` },
      v2raytun,
      happ,
      incy,
    ],
  };
}
