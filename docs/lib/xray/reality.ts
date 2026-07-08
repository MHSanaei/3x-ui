// Pure REALITY helpers: X25519 keygen (WebCrypto) and server/client config
// templating. No React/DOM — runs in the browser and in vitest (Node 22+,
// which exposes globalThis.crypto.subtle with X25519).

import { bytesToBase64Url } from './base64';
import { buildVless } from './links';

export interface X25519KeyPair {
  /** base64url-encoded 32-byte private scalar (xray's format). */
  privateKey: string;
  /** base64url-encoded 32-byte public key. */
  publicKey: string;
}

export function isX25519Available(): boolean {
  return typeof globalThis.crypto !== 'undefined' && !!globalThis.crypto.subtle;
}

/**
 * Generate an X25519 keypair and return raw 32-byte keys as base64url, matching
 * the output of `xray x25519`. The private key cannot be exported as 'raw' in
 * WebCrypto, so we export PKCS#8 and take the final 32 bytes (the scalar).
 *
 * `subtle` is injectable so tests can run deterministically against a stub.
 */
export async function generateX25519KeyPair(
  subtle: SubtleCrypto = globalThis.crypto.subtle,
): Promise<X25519KeyPair> {
  const pair = (await subtle.generateKey({ name: 'X25519' }, true, [
    'deriveBits',
  ])) as CryptoKeyPair;

  const rawPublic = new Uint8Array(await subtle.exportKey('raw', pair.publicKey));
  const pkcs8 = new Uint8Array(await subtle.exportKey('pkcs8', pair.privateKey));
  const rawPrivate = pkcs8.slice(pkcs8.length - 32);

  return {
    privateKey: bytesToBase64Url(rawPrivate),
    publicKey: bytesToBase64Url(rawPublic),
  };
}

export function randomShortId(byteLength = 4): string {
  const bytes = new Uint8Array(byteLength);
  globalThis.crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

export function randomUuid(): string {
  return globalThis.crypto.randomUUID();
}

export interface RealityConfig {
  address: string;
  port: number;
  uuid: string;
  /** Camouflage target, e.g. `www.microsoft.com:443`. */
  dest: string;
  /** SNI / serverNames; the first is used in the client link. */
  serverNames: string[];
  shortIds: string[];
  privateKey: string;
  publicKey: string;
  fingerprint: string;
  spiderX: string;
  flow: string;
}

/** Server-side VLESS + REALITY inbound (Xray config shape). */
export function realityServerInbound(c: RealityConfig): unknown {
  return {
    listen: null,
    port: c.port,
    protocol: 'vless',
    settings: {
      clients: [{ id: c.uuid, flow: c.flow }],
      decryption: 'none',
    },
    streamSettings: {
      network: 'tcp',
      security: 'reality',
      realitySettings: {
        show: false,
        dest: c.dest,
        xver: 0,
        serverNames: c.serverNames,
        privateKey: c.privateKey,
        shortIds: c.shortIds,
        fingerprint: c.fingerprint,
      },
    },
    sniffing: { enabled: true, destOverride: ['http', 'tls', 'quic'] },
  };
}

/** Client `vless://` share link carrying the public REALITY parameters. */
export function realityClientLink(c: RealityConfig): string {
  return buildVless({
    credential: c.uuid,
    address: c.address,
    port: c.port,
    params: {
      type: 'tcp',
      security: 'reality',
      pbk: c.publicKey,
      fp: c.fingerprint,
      sni: c.serverNames[0] ?? '',
      sid: c.shortIds[0] ?? '',
      spx: c.spiderX,
      flow: c.flow,
    },
    name: `${c.address}-reality`,
  });
}
