import { formatInboundLabel } from '@/lib/inbounds/label';
import { preferPublicHost, resolveShareHost } from '@/lib/xray/inbound-link';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

export function findAmneziaWGInbound(
  client: ClientRecord | null | undefined,
  inboundsById: Record<number, InboundOption>,
): InboundOption | undefined {
  return (client?.inboundIds || [])
    .map((id) => inboundsById[id])
    .find((ib) => ib?.protocol === 'amneziawg');
}

export function buildAmneziaWGClientConfig(
  client: ClientRecord,
  inbound: InboundOption | undefined,
  host = window.location.hostname,
  publicHost = '',
): string {
  const endpointHost = resolveShareHost(inbound ?? {}, inbound?.nodeAddress ?? '', preferPublicHost(host, publicHost));
  const server = inbound?.awgServer;
  const address = client.allowedIPs || '10.8.1.2/32';
  const endpoint = `${endpointHost}:${inbound?.port || server?.serverPort || ''}`;
  const inboundName = inbound ? formatInboundLabel(inbound.tag, inbound.remark) : '';
  const remark = [inboundName, client.email, client.comment].filter(Boolean).join(' - ');

  const dns = [server?.primaryDns, server?.secondaryDns].filter(Boolean).join(', ') || '1.1.1.1, 1.0.0.1';

  const lines = [
    '[Interface]',
    `PrivateKey = ${client.privateKey || ''}`,
    `Address = ${address}`,
    `DNS = ${dns}`,
  ];

  if (server) {
    lines.push(`Jc = ${server.jc}`);
    lines.push(`Jmin = ${server.jmin}`);
    lines.push(`Jmax = ${server.jmax}`);
    lines.push(`S1 = ${server.s1}`);
    lines.push(`S2 = ${server.s2}`);
    lines.push(`S3 = ${server.s3}`);
    lines.push(`S4 = ${server.s4}`);
    lines.push(`H1 = ${server.h1}`);
    lines.push(`H2 = ${server.h2}`);
    lines.push(`H3 = ${server.h3}`);
    lines.push(`H4 = ${server.h4}`);
  }

  lines.push('');
  if (remark) lines.push(`# ${remark}`);
  lines.push('[Peer]', `PublicKey = ${server?.publicKey || ''}`);
  if (client.preSharedKey) lines.push(`PresharedKey = ${client.preSharedKey}`);
  lines.push('AllowedIPs = 0.0.0.0/0, ::/0', `Endpoint = ${endpoint}`);
  lines.push('PersistentKeepalive = 25');
  return lines.join('\n');
}
