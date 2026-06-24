import type { ExternalProxyEntry } from '@/schemas/protocols/stream/external-proxy';
import type { HostFormValues } from '@/schemas/api/host';

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
      host.pinnedPeerCertSha256 && host.pinnedPeerCertSha256.length > 0 ? host.pinnedPeerCertSha256 : undefined,
    verifyPeerCertByName: host.verifyPeerCertByName || undefined,
    echConfigList: host.echConfigList || undefined,
  };
}
