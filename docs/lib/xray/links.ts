// Pure parsing/building of Xray share links (vless / vmess / trojan / ss).
// No React/DOM — runs in the browser and in vitest (Node) alike.

import { base64ToText, textToBase64 } from './base64';

export type Protocol = 'vless' | 'vmess' | 'trojan' | 'ss';

export interface ParsedLink {
  protocol: Protocol;
  /** Remark / fragment label. */
  name: string;
  address: string;
  port: number;
  /** UUID (vless/vmess), password (trojan), or `method:password` (ss). */
  credential: string;
  /** All remaining parameters, for display. */
  params: Record<string, string>;
}

export function detectProtocol(link: string): Protocol | null {
  const scheme = link.trim().slice(0, link.indexOf('://')).toLowerCase();
  if (scheme === 'vless' || scheme === 'vmess' || scheme === 'trojan' || scheme === 'ss') {
    return scheme;
  }
  return null;
}

export function parseLink(link: string): ParsedLink {
  const trimmed = link.trim();
  const protocol = detectProtocol(trimmed);
  switch (protocol) {
    case 'vless':
    case 'trojan':
      return parseUserinfoLink(trimmed, protocol);
    case 'vmess':
      return parseVmess(trimmed);
    case 'ss':
      return parseShadowsocks(trimmed);
    default:
      throw new Error(
        `Unsupported or invalid link. Expected vless://, vmess://, trojan://, or ss://`,
      );
  }
}

// vless:// and trojan:// share the `cred@host:port?params#name` structure.
function parseUserinfoLink(link: string, protocol: 'vless' | 'trojan'): ParsedLink {
  const url = new URL(link);
  const params: Record<string, string> = {};
  url.searchParams.forEach((value, key) => {
    params[key] = value;
  });
  return {
    protocol,
    name: safeDecode(url.hash.replace(/^#/, '')),
    address: stripBrackets(url.hostname),
    port: Number(url.port) || 0,
    credential: safeDecode(url.username),
    params,
  };
}

function parseVmess(link: string): ParsedLink {
  const payload = link.slice('vmess://'.length);
  let obj: Record<string, unknown>;
  try {
    obj = JSON.parse(base64ToText(payload)) as Record<string, unknown>;
  } catch {
    throw new Error('Invalid vmess link: payload is not base64-encoded JSON.');
  }
  const reserved = new Set(['ps', 'add', 'port', 'id', 'v']);
  const params: Record<string, string> = {};
  for (const [key, value] of Object.entries(obj)) {
    if (reserved.has(key) || value === undefined || value === null || value === '') continue;
    params[key] = String(value);
  }
  return {
    protocol: 'vmess',
    name: String(obj.ps ?? ''),
    address: String(obj.add ?? ''),
    port: Number(obj.port) || 0,
    credential: String(obj.id ?? ''),
    params,
  };
}

function parseShadowsocks(link: string): ParsedLink {
  const rest = link.slice('ss://'.length);
  const hashIndex = rest.indexOf('#');
  const name = hashIndex >= 0 ? safeDecode(rest.slice(hashIndex + 1)) : '';
  const body = hashIndex >= 0 ? rest.slice(0, hashIndex) : rest;

  if (body.includes('@')) {
    // SIP002: ss://<userinfo>@host:port[/][?plugin=...]
    const atIndex = body.lastIndexOf('@');
    const userinfo = body.slice(0, atIndex);
    const hostPart = body.slice(atIndex + 1);
    const credential = userinfo.includes(':') ? safeDecode(userinfo) : tryBase64(userinfo);
    const { address, port, params } = parseHostPortQuery(hostPart);
    return { protocol: 'ss', name, address, port, credential, params };
  }

  // Legacy: ss://base64(method:password@host:port)
  const decoded = base64ToText(body);
  const atIndex = decoded.lastIndexOf('@');
  if (atIndex < 0) throw new Error('Invalid ss link: missing host.');
  const credential = decoded.slice(0, atIndex);
  const { address, port } = parseHostPortQuery(decoded.slice(atIndex + 1));
  return { protocol: 'ss', name, address, port, credential, params: {} };
}

function parseHostPortQuery(input: string): {
  address: string;
  port: number;
  params: Record<string, string>;
} {
  const queryIndex = input.search(/[/?]/);
  const hostPort = queryIndex >= 0 ? input.slice(0, queryIndex) : input;
  const query = queryIndex >= 0 ? input.slice(queryIndex).replace(/^\/?\??/, '') : '';
  const match = /^(\[[^\]]+\]|[^:]+):(\d+)$/.exec(hostPort);
  if (!match) throw new Error('Invalid host:port.');
  const params: Record<string, string> = {};
  if (query) {
    new URLSearchParams(query).forEach((value, key) => {
      params[key] = value;
    });
  }
  return { address: stripBrackets(match[1]), port: Number(match[2]), params };
}

// ---- Builders -------------------------------------------------------------

export interface VlessTrojanInput {
  credential: string;
  address: string;
  port: number;
  params?: Record<string, string>;
  name?: string;
}

export function buildVless(input: VlessTrojanInput): string {
  return buildUserinfoLink('vless', input);
}

export function buildTrojan(input: VlessTrojanInput): string {
  return buildUserinfoLink('trojan', input);
}

function buildUserinfoLink(scheme: 'vless' | 'trojan', input: VlessTrojanInput): string {
  const search = new URLSearchParams(
    Object.entries(input.params ?? {}).filter(([, v]) => v !== '' && v != null),
  ).toString();
  const host = input.address.includes(':') ? `[${input.address}]` : input.address;
  const query = search ? `?${search}` : '';
  const fragment = input.name ? `#${encodeURIComponent(input.name)}` : '';
  return `${scheme}://${encodeURIComponent(input.credential)}@${host}:${input.port}${query}${fragment}`;
}

export function buildVmess(obj: Record<string, string | number>): string {
  return `vmess://${textToBase64(JSON.stringify({ v: '2', ...obj }))}`;
}

export interface ShadowsocksInput {
  method: string;
  password: string;
  address: string;
  port: number;
  name?: string;
}

export function buildShadowsocks(input: ShadowsocksInput): string {
  const userinfo = textToBase64(`${input.method}:${input.password}`)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
  const host = input.address.includes(':') ? `[${input.address}]` : input.address;
  const fragment = input.name ? `#${encodeURIComponent(input.name)}` : '';
  return `ss://${userinfo}@${host}:${input.port}${fragment}`;
}

// ---- helpers --------------------------------------------------------------

function stripBrackets(host: string): string {
  return host.replace(/^\[/, '').replace(/\]$/, '');
}

function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function tryBase64(value: string): string {
  try {
    const decoded = base64ToText(value);
    return decoded.includes(':') ? decoded : value;
  } catch {
    return value;
  }
}
