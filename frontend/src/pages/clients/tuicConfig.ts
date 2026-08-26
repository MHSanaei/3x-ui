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

  let tuicSettings: {
    alpn?: string[];
    sni?: string;
    congestion_control?: string;
    udp_relay_mode?: string;
    zero_rtt_handshake?: boolean;
  } = {};
  const rawSettings = (inbound as { settings?: unknown })?.settings;
  if (typeof rawSettings === 'string') {
    try {
      tuicSettings = JSON.parse(rawSettings);
    } catch {
      tuicSettings = {};
    }
  } else if (rawSettings && typeof rawSettings === 'object') {
    tuicSettings = rawSettings as typeof tuicSettings;
  }

  const alpn =
    Array.isArray(tuicSettings.alpn) && tuicSettings.alpn.length > 0
      ? tuicSettings.alpn
      : ['h3', 'spdy/3.1'];
  const sni = tuicSettings.sni || endpointHost;
  const cc = tuicSettings.congestion_control || 'bbr';
  const udpRelay = tuicSettings.udp_relay_mode || 'native';
  const reduceRtt = tuicSettings.zero_rtt_handshake ?? true;

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
