import { formatInboundLabel } from '@/lib/inbounds/label';
import { preferPublicHost, resolveShareHost } from '@/lib/xray/inbound-link';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

export function isTuicClient(client: ClientRecord | null | undefined): boolean {
  if (!client) return false;
  return !!(client.uuid && client.password);
}

export function findTuicInbound(
  client: ClientRecord | null | undefined,
  inboundsById: Record<number, InboundOption>,
): InboundOption | undefined {
  return (client?.inboundIds || [])
    .map((id) => inboundsById[id])
    .find((ib) => ib?.protocol === 'tuic');
}

export function buildTuicClientConfig(
  client: ClientRecord,
  inbound: InboundOption | undefined,
  host = window.location.hostname,
  publicHost = '',
): string {
  const endpointHost = resolveShareHost(
    inbound ?? {},
    inbound?.nodeAddress ?? '',
    preferPublicHost(host, publicHost),
  );
  const inboundName = inbound ? formatInboundLabel(inbound.tag, inbound.remark) : '';
  const remark = [inboundName, client.email].filter(Boolean).join(' - ') || 'tuic-client';

  const tuicServer = inbound?.tuicServer;
  const alpn =
    Array.isArray(tuicServer?.alpn) && tuicServer.alpn.length > 0
      ? tuicServer.alpn
      : ['h3', 'spdy/3.1'];
  const sni = tuicServer?.sni || endpointHost;
  const cc = tuicServer?.congestion_control || 'bbr';
  const udpRelay = tuicServer?.udp_relay_mode || 'native';
  const reduceRtt = tuicServer?.zero_rtt_handshake ?? true;

  const lines = [
    `# TUIC v5 Client Configuration (Clash / Mihomo / Clash Verge)`,
    `# ${remark}`,
    `proxies:`,
    `  - name: "${remark}"`,
    `    type: tuic`,
    `    server: ${endpointHost}`,
    `    port: ${inbound?.port || 8443}`,
    `    uuid: ${client.uuid || ''}`,
    `    password: "${client.password || ''}"`,
    `    alpn:`,
    ...alpn.map((a: string) => `      - ${a}`),
    `    sni: ${sni}`,
    `    congestion-controller: ${cc}`,
    `    udp-relay-mode: ${udpRelay}`,
    `    reduce-rtt: ${reduceRtt}`,
  ];

  return lines.join('\n');
}
