import { rawOutboundToFormValues } from '@/lib/xray/outbound-form-adapter';
import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

import { MUX_PROTOCOLS } from './outbound-form-constants';

export function isMuxAllowed(protocol: string, flow: string, network: string): boolean {
  if (!MUX_PROTOCOLS.has(protocol)) return false;
  if (protocol === 'vless' && flow) return false;
  if (network === 'xhttp') return false;
  return true;
}

// Per-network bootstrap. Mirrors the legacy class constructors so the
// initial state for each transport matches what xray-core expects.
export function newStreamSlice(network: string): Record<string, unknown> {
  switch (network) {
    case 'tcp':
      return { network: 'tcp', tcpSettings: { header: { type: 'none' } } };
    case 'kcp':
      return {
        network: 'kcp',
        kcpSettings: {
          mtu: 1350, tti: 20, uplinkCapacity: 5, downlinkCapacity: 20,
          cwndMultiplier: 1, maxSendingWindow: 2097152,
        },
      };
    case 'ws':
      return {
        network: 'ws',
        wsSettings: { path: '/', host: '', headers: {}, heartbeatPeriod: 0 },
      };
    case 'grpc':
      return {
        network: 'grpc',
        grpcSettings: { serviceName: '', authority: '', multiMode: false },
      };
    case 'httpupgrade':
      return {
        network: 'httpupgrade',
        httpupgradeSettings: { path: '/', host: '', headers: {} },
      };
    case 'xhttp':
      return {
        network: 'xhttp',
        xhttpSettings: {
          path: '/', host: '', mode: '', headers: [],
          xPaddingBytes: '100-1000', scMaxEachPostBytes: '1000000',
        },
      };
    case 'hysteria':
      return {
        network: 'hysteria',
        hysteriaSettings: {
          version: 2,
          auth: '',
          udpIdleTimeout: 60,
        },
      };
    default:
      return { network: 'tcp', tcpSettings: { header: { type: 'none' } } };
  }
}

// Hysteria2 always rides its own QUIC transport with TLS — the panel never
// offers another transport or 'none' security for it.
export function hysteriaStreamSlice(): Record<string, unknown> {
  return {
    ...newStreamSlice('hysteria'),
    security: 'tls',
    tlsSettings: {
      serverName: '', alpn: ['h3'], fingerprint: '',
      echConfigList: '', verifyPeerCertByName: '', pinnedPeerCertSha256: '',
    },
  };
}

export function buildAddModeValues(): OutboundFormValues {
  return rawOutboundToFormValues({});
}
