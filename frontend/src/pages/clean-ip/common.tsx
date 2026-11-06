import type { ReactNode } from 'react';

import { HttpUtil } from '@/utils';

import { inboundDomain, type CleanIpCdn } from './scan';

export type { CleanIpCdn };

export interface CleanIpNetwork {
  ip: string;
  asn: number;
  org: string;
  country: string;
  isp: string;
}

// Cloudflare's own networks: a visitor seen from one of them is on WARP or a VPN, not an ISP.
export const CLOUDFLARE_ASNS = new Set([13335, 14789, 209242, 395747]);

export async function fetchCdns(): Promise<CleanIpCdn[]> {
  const msg = await HttpUtil.get('/panel/api/cleanip/cdns', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'cdns');
  return (msg.obj as CleanIpCdn[] | null) ?? [];
}

export async function fetchNetwork(): Promise<CleanIpNetwork> {
  const msg = await HttpUtil.get('/panel/api/cleanip/network', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'network');
  return msg.obj as CleanIpNetwork;
}

// The domain a host made for this inbound should use as SNI ('' when it has none).
export async function fetchInboundDomain(inboundId: number): Promise<string> {
  const msg = await HttpUtil.get(`/panel/api/inbounds/get/${inboundId}`, undefined, {
    silent: true,
  });
  const obj = msg?.success ? (msg.obj as { streamSettings?: unknown } | null) : null;
  return inboundDomain(obj?.streamSettings);
}

export function StepTitle({
  n,
  children,
  extra,
}: {
  n: number;
  children: ReactNode;
  extra?: ReactNode;
}) {
  return (
    <div className="clean-ip-step-title">
      <span className="clean-ip-step-num">{n}</span>
      <span className="clean-ip-step-text">{children}</span>
      {extra}
    </div>
  );
}
