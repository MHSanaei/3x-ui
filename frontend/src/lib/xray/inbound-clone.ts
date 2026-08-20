import { RandomUtil } from '@/utils';
import { createDefaultInboundSettings } from '@/lib/xray/inbound-defaults';
import { coerceInboundJsonField, type DBInbound } from '@/models/dbinbound';

/*
 * Payload for POST /panel/api/inbounds/add reproducing `dbInbound` as a
 * staged copy: fresh port, empty client list (emails are unique panel-wide
 * and UUIDs must not repeat across nodes), disabled, no tag (the backend
 * regenerates one with the correct per-node prefix), cleared listen (listen
 * addresses are node-local). `nodeId === null` targets the local panel; the
 * field is omitted from the wire payload then, matching the add-form adapter.
 */
export function buildClonePayload(dbInbound: DBInbound, port: number, nodeId: number | null) {
  let clonedSettings: string;
  try {
    const raw = { ...coerceInboundJsonField(dbInbound.settings) };
    raw.clients = [];
    clonedSettings = JSON.stringify(raw);
  } catch {
    const fallback = createDefaultInboundSettings(dbInbound.protocol);
    clonedSettings = fallback ? JSON.stringify(fallback, null, 2) : '{}';
  }
  const streamSettingsString =
    typeof dbInbound.streamSettings === 'string'
      ? dbInbound.streamSettings
      : JSON.stringify(dbInbound.streamSettings ?? {});
  const sniffingString =
    typeof dbInbound.sniffing === 'string'
      ? dbInbound.sniffing
      : JSON.stringify(dbInbound.sniffing ?? {});
  return {
    up: 0,
    down: 0,
    total: 0,
    remark: `${dbInbound.remark} (clone)`,
    enable: false,
    expiryTime: 0,
    listen: '',
    port,
    protocol: dbInbound.protocol,
    settings: clonedSettings,
    streamSettings: streamSettingsString,
    sniffing: sniffingString,
    shareAddrStrategy: dbInbound.shareAddrStrategy,
    shareAddr: dbInbound.shareAddr,
    ...(nodeId != null ? { nodeId } : {}),
  };
}

/*
 * Random clone port in the add-form's range, avoiding ports already bound on
 * the target node (client-side pre-check; the backend's node-scoped conflict
 * check stays the final arbiter). A few random tries cover the common sparse
 * case; a target so dense that those all miss falls back to a deterministic
 * scan so a free port is always found when one exists.
 */
export function pickClonePort(used: Set<number> | undefined): number {
  let port = RandomUtil.randomInteger(10000, 60000);
  if (!used) return port;
  for (let attempts = 0; attempts < 20 && used.has(port); attempts++) {
    port = RandomUtil.randomInteger(10000, 60000);
  }
  if (used.has(port)) {
    for (port = 10000; port <= 60000 && used.has(port); port++) {
      /* dense-range scan */
    }
    if (port > 60000) port = RandomUtil.randomInteger(10000, 60000);
  }
  return port;
}
