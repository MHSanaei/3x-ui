import { rawOutboundToFormValues } from '@/lib/xray/outbound-form-adapter';
import { canEnableReality, canEnableTls } from '@/lib/xray/protocol-capabilities';
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
          xPaddingBytes: '100-1000',
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

// Network change cascade: swap the per-network sub-key (tcpSettings,
// wsSettings, etc.) so the DU branch matches. Carry over the security mode
// and its settings (tlsSettings/realitySettings, including SNI serverName)
// when the new network still supports it; otherwise fall back to 'none'.
// Dropping tlsSettings here silently wiped the spoofed SNI on save (#4791).
export function applyNetworkChange(
  protocol: string,
  prevStream: Record<string, unknown> | undefined,
  next: string,
): Record<string, unknown> {
  if (next === 'hysteria') return hysteriaStreamSlice();
  const stream = prevStream ?? {};
  const currentSecurity = (stream.security as string) ?? 'none';
  const stillTls = canEnableTls({ protocol, streamSettings: { network: next, security: currentSecurity } });
  const stillReality = canEnableReality({ protocol, streamSettings: { network: next, security: currentSecurity } });
  const newSecurity =
    currentSecurity === 'tls' && !stillTls
      ? 'none'
      : currentSecurity === 'reality' && !stillReality
        ? 'none'
        : currentSecurity;
  const newStream: Record<string, unknown> = { ...newStreamSlice(next), security: newSecurity };
  if (newSecurity === 'tls' && stream.tlsSettings) newStream.tlsSettings = stream.tlsSettings;
  else if (newSecurity === 'reality' && stream.realitySettings) newStream.realitySettings = stream.realitySettings;
  return newStream;
}

export function buildAddModeValues(): OutboundFormValues {
  return rawOutboundToFormValues({});
}
