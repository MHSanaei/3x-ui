import type { ExternalProxyEntry } from '@/schemas/protocols/stream/external-proxy';
import type { HostFormValues, HostRecord } from '@/schemas/api/host';
import type { Inbound } from '@/schemas/api/inbound';
import { resolveAddr } from '@/lib/xray/inbound-link';

// The subset of a host that affects its share link. Mirrors the fields the
// backend's hostToExternalProxyMap reads.
export type HostLinkInput = Pick<
  HostFormValues,
  | 'security'
  | 'address'
  | 'port'
  | 'remark'
  | 'sni'
  | 'alpn'
  | 'fingerprint'
  | 'pinnedPeerCertSha256'
  | 'verifyPeerCertByName'
  | 'echConfigList'
  | 'overrideSniFromAddress'
  | 'keepSniBlank'
  | 'vlessRoute'
>;

// hostToExternalProxyEntry projects a host onto the ExternalProxyEntry shape the
// share-link preview generators already understand — the frontend mirror of the
// backend's hostToExternalProxyMap. security "reality"/"same" keep the inbound's
// base TLS (forceTls "same"); the preview falls back to port 443 when the host
// inherits the inbound port (port 0).
export function hostToExternalProxyEntry(host: HostLinkInput): ExternalProxyEntry {
  const forceTls = host.security === 'tls' || host.security === 'none' ? host.security : 'same';

  let sni: string | undefined;
  if (host.keepSniBlank) {
    sni = undefined;
  } else if (host.overrideSniFromAddress) {
    sni = host.address || undefined;
  } else {
    sni = host.sni || undefined;
  }

  return {
    forceTls,
    dest: host.address || '',
    port: host.port && host.port > 0 ? host.port : 443,
    remark: host.remark || '',
    sni,
    fingerprint: host.fingerprint,
    alpn: host.alpn && host.alpn.length > 0 ? host.alpn : undefined,
    pinnedPeerCertSha256:
      host.pinnedPeerCertSha256 && host.pinnedPeerCertSha256.length > 0
        ? host.pinnedPeerCertSha256
        : undefined,
    verifyPeerCertByName: host.verifyPeerCertByName || undefined,
    echConfigList: host.echConfigList || undefined,
    vlessRoute: host.vlessRoute || undefined,
  };
}

function splitAdvertisedHost(value: string, inboundPort: number): [string, number] {
  const host = value.trim();
  if (host.startsWith('[')) {
    const close = host.indexOf(']');
    if (close > 0) {
      const port = host.slice(close + 1).match(/^:(\d+)$/)?.[1];
      return [host.slice(1, close), port ? Number(port) : inboundPort];
    }
  }
  const match = host.match(/^([^:]*):(\d+)$/);
  return match ? [match[1], Number(match[2])] : [host, inboundPort];
}

export function withMtprotoHostEndpoints(
  inbound: Inbound,
  inboundId: number,
  records: HostRecord[],
  hostOverride: string,
  fallbackHostname: string,
): Inbound {
  if (inbound.protocol !== 'mtproto') return inbound;
  const endpoints: ExternalProxyEntry[] = [];
  for (const record of records) {
    if (
      record.isDisabled ||
      !record.inboundIds.includes(inboundId) ||
      record.excludeFromSubTypes?.includes('raw')
    ) {
      continue;
    }
    for (const value of record.hosts) {
      const [dest, port] = splitAdvertisedHost(value, inbound.port);
      endpoints.push({
        forceTls: 'same',
        dest: dest || resolveAddr(inbound, hostOverride, fallbackHostname),
        port,
        remark: record.remark || '',
      });
    }
  }
  if (endpoints.length === 0) return inbound;
  return {
    ...inbound,
    streamSettings: { ...inbound.streamSettings, externalProxy: endpoints },
  } as Inbound;
}
