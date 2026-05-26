import type { Inbound } from '@/schemas/api/inbound';
import { coerceInboundJsonField } from '@/models/dbinbound';

export interface DbInboundLike {
  port: number;
  listen: string;
  protocol: string;
  settings: unknown;
  streamSettings: unknown;
  sniffing: unknown;
  tag?: string;
  remark?: string;
  enable?: boolean;
  expiryTime?: number;
  up?: number;
  down?: number;
  total?: number;
}

export function inboundFromDb(raw: DbInboundLike): Inbound {
  const settings = coerceInboundJsonField(raw.settings);
  const streamSettings = coerceInboundJsonField(raw.streamSettings);
  const sniffing = coerceInboundJsonField(raw.sniffing);
  return {
    protocol: raw.protocol,
    port: raw.port,
    listen: raw.listen ?? '',
    tag: raw.tag ?? '',
    remark: raw.remark ?? '',
    enable: raw.enable ?? true,
    expiryTime: raw.expiryTime ?? 0,
    up: raw.up ?? 0,
    down: raw.down ?? 0,
    total: raw.total ?? 0,
    settings,
    streamSettings,
    sniffing,
  } as unknown as Inbound;
}
