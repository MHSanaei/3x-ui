import { Tag } from 'antd';
import { Base64 } from '@/utils';

/* Shared parsing + rendering for the "protocol / transport / security"
   labels shown above share links in the QR modal, the client info modal
   and the subscription page. Keeping it in one place means the colour
   scheme and the email/stats stripping stay identical across all three. */

export interface LinkParts {
  protocol: string;
  network: string;
  security: string;
  remark: string;
  port: string;
}

const PROTOCOL_LABELS: Record<string, string> = {
  vless: 'Vless',
  vmess: 'Vmess',
  trojan: 'Trojan',
  ss: 'Shadowsocks',
  shadowsocks: 'Shadowsocks',
  hysteria2: 'Hysteria2',
  hy2: 'Hysteria2',
  hysteria: 'Hysteria',
  wireguard: 'WireGuard',
  wg: 'WireGuard',
};

const PROTOCOL_COLORS: Record<string, string> = {
  Vless: 'geekblue',
  Vmess: 'blue',
  Trojan: 'volcano',
  Shadowsocks: 'purple',
  Hysteria: 'magenta',
  Hysteria2: 'magenta',
  WireGuard: 'cyan',
};

const SECURITY_COLORS: Record<string, string> = {
  TLS: 'green',
  XTLS: 'green',
  REALITY: 'purple',
};

const TRANSPORT_COLOR = 'gold';

const TAG_STYLE = { marginInlineEnd: 0, fontWeight: 600, letterSpacing: '0.3px' };

/* Pull protocol, transport, security plus the remark and port out of a share
   link. vless/trojan carry network+security as `type`/`security` query params
   and the remark in the URL hash; vmess packs them into the base64 JSON as
   `net`/`tls`/`ps`/`port`. Returns null when the scheme is unknown or the
   payload can't be parsed, so callers fall back to "Link N".

   The remark is shown verbatim: the panel displays the subscription's clean
   (name-only) remarks — the per-client traffic/expiry info is rendered only
   into the body a client app imports, so there is nothing to strip here. */
export function parseLinkParts(link: string): LinkParts | null {
  const trimmed = link.trim();
  const scheme = /^([a-z0-9]+):\/\//i.exec(trimmed)?.[1]?.toLowerCase() ?? '';
  if (!scheme) return null;
  const protocol = PROTOCOL_LABELS[scheme] ?? scheme.charAt(0).toUpperCase() + scheme.slice(1);
  let network = '';
  let security = '';
  let remark = '';
  let port = '';
  if (scheme === 'vmess') {
    try {
      const json = JSON.parse(Base64.decode(trimmed.slice('vmess://'.length).split('#')[0])) as {
        net?: string;
        tls?: string;
        ps?: string;
        port?: string | number;
      };
      network = json.net ?? '';
      security = json.tls ?? '';
      remark = typeof json.ps === 'string' ? json.ps : '';
      port = json.port != null ? String(json.port) : '';
    } catch { /* unparseable payload, fall back to protocol only */ }
  } else {
    try {
      const url = new URL(trimmed);
      network = url.searchParams.get('type') ?? '';
      security = url.searchParams.get('security') ?? '';
      port = url.port;
      const hash = url.hash.replace(/^#/, '');
      try { remark = decodeURIComponent(hash); } catch { remark = hash; }
    } catch { /* not URL-shaped, fall back to protocol only */ }
  }
  if (security === 'none') security = '';
  return {
    protocol,
    network: network.toUpperCase(),
    security: security.toUpperCase(),
    remark: remark.trim(),
    port,
  };
}

/* The inbound remark and port joined as they appear after the tags, e.g.
   "22:10452". Either piece may be empty. */
export function linkMetaText(parts: LinkParts): string {
  return [parts.remark, parts.port].filter(Boolean).join(':');
}

export function LinkTags({ parts }: { parts: LinkParts }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
      <Tag color={PROTOCOL_COLORS[parts.protocol]} style={TAG_STYLE}>{parts.protocol}</Tag>
      {parts.network && <Tag color={TRANSPORT_COLOR} style={TAG_STYLE}>{parts.network}</Tag>}
      {parts.security && (
        <Tag color={SECURITY_COLORS[parts.security]} style={TAG_STYLE}>{parts.security}</Tag>
      )}
    </span>
  );
}
