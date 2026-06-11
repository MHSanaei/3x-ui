import type { Inbound } from '@/schemas/api/inbound';
import { InboundSettingsSchema } from '@/schemas/protocols/inbound';
import { coerceInboundJsonField } from '@/models/dbinbound';

import { fillStreamDefaults } from './stream-defaults';

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
  shareAddrStrategy?: string;
  shareAddr?: string;
}

function fillProtocolSettingsDefaults(protocol: string, settings: Record<string, unknown>): Record<string, unknown> {
  const parsed = InboundSettingsSchema.safeParse({ protocol, settings });
  if (parsed.success) {
    const tagged = parsed.data as { settings: Record<string, unknown> };
    return { ...tagged.settings };
  }
  return settings;
}

export function inboundFromDb(raw: DbInboundLike): Inbound {
  const rawSettings = coerceInboundJsonField(raw.settings);
  const settings = fillProtocolSettingsDefaults(raw.protocol, rawSettings);
  const streamSettingsRaw = coerceInboundJsonField(raw.streamSettings);
  const sniffing = coerceInboundJsonField(raw.sniffing);
  const streamSettings = Object.keys(streamSettingsRaw).length === 0
    ? streamSettingsRaw
    : fillStreamDefaults(streamSettingsRaw);
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
    shareAddrStrategy: raw.shareAddrStrategy ?? 'node',
    shareAddr: raw.shareAddr ?? '',
    settings,
    streamSettings,
    sniffing,
  } as unknown as Inbound;
}
