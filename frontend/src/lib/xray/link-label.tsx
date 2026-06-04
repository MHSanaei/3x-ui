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

/* Strip the client email and the optional traffic/expiry decorations the
   panel appends to a remark (e.g. "5.23GB📊", "30D⏳", "⛔️N/A") together
   with any separator chars left dangling, so the label shows just the
   inbound remark. The email is known from the client record, so it can be
   removed even though its position in the composed remark depends on the
   panel's remark-model settings. */
function cleanRemark(remark: string, email: string): string {
  let r = remark
    .replace(/⛔️?N\/A/gu, '')
    .replace(/[0-9][0-9A-Za-z.,]*[📊⏳]/gu, '');
  if (email) {
    const esc = email.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    r = r.replace(new RegExp(`[\\s\\-_.|,@]*${esc}`, 'g'), '');
  }
  return r.replace(/^[\s\-_.|,@]+|[\s\-_.|,@]+$/gu, '').trim();
}

/* Pull protocol, transport, security plus the inbound remark and port out
   of a share link. vless/trojan carry network+security as `type`/`security`
   query params and the remark in the URL hash; vmess packs them into the
   base64 JSON as `net`/`tls`/`ps`/`port`. Returns null when the scheme is
   unknown or the payload can't be parsed, so callers fall back to "Link N". */
export function parseLinkParts(link: string, email = ''): LinkParts | null {
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
    remark: cleanRemark(remark, email),
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
