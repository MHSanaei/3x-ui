import type { InboundFormValues } from '@/schemas/forms/inbound-form';

export interface InboundLintIssue {
  key: string;
  level: 'warning' | 'error';
  message: string;
}

function asRecord(v: unknown): Record<string, unknown> {
  return v && typeof v === 'object' && !Array.isArray(v) ? v as Record<string, unknown> : {};
}

function asStringArray(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [];
}

function nonEmptyString(v: unknown): string {
  return typeof v === 'string' ? v.trim() : '';
}

function isRootOrEmptyPath(path: unknown): boolean {
  const p = nonEmptyString(path);
  return p === '' || p === '/';
}

export function lintInboundConfig(values: Partial<InboundFormValues> | undefined): InboundLintIssue[] {
  if (!values) return [];
  const issues: InboundLintIssue[] = [];
  const protocol = values.protocol;
  const stream = asRecord(values.streamSettings);
  const network = nonEmptyString(stream.network);
  const security = nonEmptyString(stream.security) || 'none';

  if (security === 'reality') {
    const reality = asRecord(stream.realitySettings);
    const settings = asRecord(reality.settings);
    const shortIds = asStringArray(reality.shortIds).filter(Boolean);
    const spiderX = nonEmptyString(settings.spiderX);
    const fingerprint = nonEmptyString(settings.fingerprint);
    const target = nonEmptyString(reality.target);

    if (shortIds.length < 2) {
      issues.push({
        key: 'reality-shortids',
        level: 'warning',
        message: 'Reality should use multiple random shortIds so every client does not share one identifier.',
      });
    }
    if (spiderX === '' || spiderX === '/') {
      issues.push({
        key: 'reality-spiderx',
        level: 'warning',
        message: 'Reality spiderX is default or empty. Use a randomized non-root path per deployment/client.',
      });
    }
    if (fingerprint === 'chrome') {
      issues.push({
        key: 'reality-fingerprint',
        level: 'warning',
        message: 'Reality uTLS is fixed to chrome. Mix browser fingerprints or use randomized to avoid monoculture.',
      });
    }
    if (target === 'images.apple.com:443') {
      issues.push({
        key: 'reality-default-target',
        level: 'warning',
        message: 'Reality target still uses the old default images.apple.com:443. Prefer scanning and selecting a fresh feasible target.',
      });
    }
    if (network === 'tcp' && protocol === 'vless') {
      const settingsRoot = asRecord(values.settings);
      const clients = Array.isArray(settingsRoot.clients) ? settingsRoot.clients : [];
      const missingVision = clients.some((client) => asRecord(client).flow !== 'xtls-rprx-vision');
      if (clients.length > 0 && missingVision) {
        issues.push({
          key: 'reality-vision',
          level: 'warning',
          message: 'VLESS TCP Reality clients should use xtls-rprx-vision unless you have a compatibility reason.',
        });
      }
    }
  }

  if (network === 'xhttp') {
    const xhttp = asRecord(stream.xhttpSettings);
    if (isRootOrEmptyPath(xhttp.path)) {
      issues.push({
        key: 'xhttp-root-path',
        level: 'warning',
        message: 'XHTTP path is root or empty. Use a plausible non-root web path.',
      });
    }
    const interval = nonEmptyString(xhttp.scMinPostsIntervalMs);
    if (interval === '30') {
      issues.push({
        key: 'xhttp-interval-default',
        level: 'warning',
        message: 'XHTTP scMinPostsIntervalMs=30 is a known stable fingerprint. Leave it empty or use a randomized range.',
      });
    }
  }

  if (network === 'ws') {
    const ws = asRecord(stream.wsSettings);
    if (isRootOrEmptyPath(ws.path)) {
      issues.push({
        key: 'ws-root-path',
        level: 'warning',
        message: 'WebSocket path is root or empty. Use a realistic non-root endpoint.',
      });
    }
  }

  if (network === 'httpupgrade') {
    const hu = asRecord(stream.httpupgradeSettings);
    if (isRootOrEmptyPath(hu.path)) {
      issues.push({
        key: 'httpupgrade-root-path',
        level: 'warning',
        message: 'HTTPUpgrade path is root or empty. Use a realistic non-root endpoint.',
      });
    }
  }

  if (protocol === 'hysteria') {
    const hysteria = asRecord(stream.hysteriaSettings);
    const finalmask = asRecord(stream.finalmask);
    const udpMasks = Array.isArray(finalmask.udp) ? finalmask.udp : [];
    const quicParams = asRecord(finalmask.quicParams);
    const udpHop = asRecord(quicParams.udpHop);
    const hasSalamander = udpMasks.some((mask) => asRecord(mask).type === 'salamander');

    if (!asRecord(hysteria.masquerade).type) {
      issues.push({
        key: 'hysteria-masquerade',
        level: 'warning',
        message: 'Hysteria2 has no active masquerade. Enable a file, reverse-proxy, or string masquerade.',
      });
    }
    if (!hasSalamander) {
      issues.push({
        key: 'hysteria-obfs',
        level: 'warning',
        message: 'Hysteria2 has no Salamander/Gecko UDP mask. Add one under FinalMask for QUIC obfuscation.',
      });
    }
    if (!nonEmptyString(udpHop.ports)) {
      issues.push({
        key: 'hysteria-udphop',
        level: 'warning',
        message: 'Hysteria2 UDP port hopping is off. Enable udpHop when the deployment can expose a UDP port range.',
      });
    }
  }

  const tls = asRecord(stream.tlsSettings);
  const tlsSettings = asRecord(tls.settings);
  if (security === 'tls' && tlsSettings.allowInsecure === true) {
    issues.push({
      key: 'tls-allow-insecure',
      level: 'error',
      message: 'TLS allowInsecure is enabled. This weakens authentication and is easy to abuse.',
    });
  }

  return issues;
}
