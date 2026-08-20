import { formatInboundLabel } from '@/lib/inbounds/label';
import { preferPublicHost, resolveShareHost } from '@/lib/xray/inbound-link';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

// AmneziaWG clients are wire-identical to WireGuard clients (same
// privateKey/publicKey/allowedIPs/preSharedKey/keepAlive fields on
// model.Client — see wireguardConfig.ts's isWireguardClient), so this duck
// type can't tell the two protocols apart on its own; findAmneziaWGInbound's
// protocol==='amneziawg' filter below is what actually disambiguates.
export function isAmneziaWGClient(client: ClientRecord | null | undefined): boolean {
  if (!client) return false;
  return !!(client.privateKey || client.publicKey || client.allowedIPs || client.preSharedKey || client.keepAlive);
}

export function findAmneziaWGInbound(
  client: ClientRecord | null | undefined,
  inboundsById: Record<number, InboundOption>,
): InboundOption | undefined {
  return (client?.inboundIds || [])
    .map((id) => inboundsById[id])
    .find((ib) => ib?.protocol === 'amneziawg');
}

// h4Line renders one H magic-header line, matching the Go backend's
// hOrDefault fallback (blank -> the classic 1/2/3/4 WireGuard message type).
function hLine(key: string, value: string | undefined, fallback: string): string {
  return `${key} = ${value && value.trim() !== '' ? value : fallback}`;
}

export function buildAmneziaWGClientConfig(
  client: ClientRecord,
  inbound: InboundOption | undefined,
  host = window.location.hostname,
  publicHost = '',
): string {
  const server = inbound?.awgServer;
  const endpointHost = resolveShareHost(inbound ?? {}, inbound?.nodeAddress ?? '', preferPublicHost(host, publicHost));
  const address = client.allowedIPs || '10.8.1.2/32';
  const endpoint = `${endpointHost}:${inbound?.port || ''}`;
  const inboundName = inbound ? formatInboundLabel(inbound.tag, inbound.remark) : '';
  const remark = [inboundName, client.email, client.comment].filter(Boolean).join(' - ');

  // These land unescaped in [Interface]; a newline here would inject a
  // config line (e.g. a rogue PostUp) into the downloaded .conf.
  const privateKey = client.privateKey || client.password || '';
  for (const v of [privateKey, server?.primaryDns ?? '', server?.secondaryDns ?? '', remark]) {
    if (/[\r\n]/.test(v)) return '';
  }

  const dnsParts = [server?.primaryDns, server?.secondaryDns].filter((v) => !!v && v.trim() !== '');
  const lines = ['[Interface]', `PrivateKey = ${privateKey}`, `Address = ${address}`];
  if (dnsParts.length > 0) lines.push(`DNS = ${dnsParts.join(', ')}`);
  if (server?.mtu && server.mtu > 0) lines.push(`MTU = ${server.mtu}`);

  // AmneziaWG obfuscation parameters — must match the server's values.
  lines.push(`Jc = ${server?.jc ?? 5}`);
  lines.push(`Jmin = ${server?.jmin ?? 10}`);
  lines.push(`Jmax = ${server?.jmax ?? 50}`);
  lines.push(`S1 = ${server?.s1 ?? 30}`);
  lines.push(`S2 = ${server?.s2 ?? 45}`);
  if (server?.s3) lines.push(`S3 = ${server.s3}`);
  if (server?.s4) lines.push(`S4 = ${server.s4}`);
  lines.push(hLine('H1', server?.h1, '1'));
  lines.push(hLine('H2', server?.h2, '2'));
  lines.push(hLine('H3', server?.h3, '3'));
  lines.push(hLine('H4', server?.h4, '4'));
  if (server?.i1) lines.push(`I1 = ${server.i1}`);
  if (server?.i2) lines.push(`I2 = ${server.i2}`);
  if (server?.i3) lines.push(`I3 = ${server.i3}`);
  if (server?.i4) lines.push(`I4 = ${server.i4}`);
  if (server?.i5) lines.push(`I5 = ${server.i5}`);
  // AmneziaWG 3.0 fields -- HeaderProtectionKey especially must match the
  // server's value exactly, or every handshake fails outright (not just
  // weaker obfuscation).
  if (server?.headerProtectionKey) lines.push(`HeaderProtectionKey = ${server.headerProtectionKey}`);
  if (server?.contentPaddingAddition) lines.push(`ContentPaddingAddition = ${server.contentPaddingAddition}`);
  // AmneziaWG 3.1 -- RandomTrailers especially must match the server's
  // value: amneziawg-go only accepts an oversized (trailer-padded) packet
  // when the RECEIVING side's own RandomTrailers is also on, so a one-sided
  // setting makes that side's packets start getting silently dropped.
  if (server?.randomTrailers) lines.push('RandomTrailers = true');
  if (server?.disableCookies) lines.push('DisableCookies = true');

  lines.push('');
  if (remark) lines.push(`# ${remark}`);
  lines.push('[Peer]', `PublicKey = ${server?.publicKey || ''}`);
  if (client.preSharedKey) lines.push(`PresharedKey = ${client.preSharedKey}`);
  lines.push('AllowedIPs = 0.0.0.0/0, ::/0', `Endpoint = ${endpoint}`);
  if (client.keepAlive && client.keepAlive > 0) lines.push(`PersistentKeepalive = ${client.keepAlive}`);
  return lines.join('\n');
}
