import { z } from 'zod';
import type { InboundOption } from '@/schemas/client';

export const ActiveProfileTreeSchema = z.record(
  z.string(),
  z.object({
    supported: z.boolean(),
    observedAt: z.number(),
    clients: z.record(z.string(), z.array(z.string())),
  }),
);
export type ActiveProfileTree = z.infer<typeof ActiveProfileTreeSchema>;
const AUTHENTICATED_XRAY_PROTOCOLS = new Set([
  'vless',
  'vmess',
  'trojan',
  'shadowsocks',
  'hysteria',
  'http',
  'socks',
  'masque',
]);

// Match both hosting node and tag: two nodes can use the same tag, and two
// different clients can share one public source IP. Neither implies attribution.
export function resolveClientActiveProfiles(
  email: string,
  ids: number[],
  inbounds: Record<number, InboundOption>,
  tree?: ActiveProfileTree,
) {
  const activeIds: number[] = [];
  const seen = new Set<string>();
  let unknown = false;
  for (const id of ids) {
    const inbound = inbounds[id];
    const guid =
      inbound?.originNodeGuid || (inbound?.nodeId != null ? `node:${inbound.nodeId}` : '');
    const snapshot = guid && tree && Object.hasOwn(tree, guid) ? tree[guid] : undefined;
    if (
      !inbound?.tag ||
      !snapshot?.supported ||
      !AUTHENTICATED_XRAY_PROTOCOLS.has(inbound.protocol ?? '')
    ) {
      unknown = true;
      continue;
    }
    const key = `${guid}\0${inbound.tag}`;
    const tags = Object.hasOwn(snapshot.clients, email) ? snapshot.clients[email] : undefined;
    if (!seen.has(key) && tags?.includes(inbound.tag)) {
      activeIds.push(id);
      seen.add(key);
    }
  }
  return { activeIds, unknown };
}
