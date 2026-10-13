// Sidecar bridges for these protocols never put the client email in the access log.
const UNLOGGED_PROTOCOLS: ReadonlySet<string> = new Set(['tuic', 'mtproto']);

// attributed holds the tags the access log paired with this client, if any.
export function isClientOnlineOnInbound(opts: {
  attributed: ReadonlySet<string> | undefined;
  tag: string | undefined;
  protocol: string;
  inboundActive: boolean;
}): boolean {
  const { attributed, tag, protocol, inboundActive } = opts;
  if (!attributed || !tag || UNLOGGED_PROTOCOLS.has(protocol)) return inboundActive;
  return attributed.has(tag);
}
