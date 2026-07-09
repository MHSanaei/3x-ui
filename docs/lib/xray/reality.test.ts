import { describe, it, expect } from 'vitest';
import {
  generateX25519KeyPair,
  isX25519Available,
  randomShortId,
  realityClientLink,
  realityServerInbound,
  type RealityConfig,
} from './reality';
import { base64UrlToBytes } from './base64';
import { parseLink } from './links';

describe('X25519 keygen', () => {
  it('is available in this environment (Node 22+ / modern browser)', () => {
    expect(isX25519Available()).toBe(true);
  });

  it('produces 32-byte base64url keys that differ', async () => {
    const { privateKey, publicKey } = await generateX25519KeyPair();
    expect(privateKey).not.toMatch(/[+/=]/);
    expect(publicKey).not.toMatch(/[+/=]/);
    expect(base64UrlToBytes(privateKey)).toHaveLength(32);
    expect(base64UrlToBytes(publicKey)).toHaveLength(32);
    expect(privateKey).not.toEqual(publicKey);
  });

  it('generates a fresh keypair each call', async () => {
    const a = await generateX25519KeyPair();
    const b = await generateX25519KeyPair();
    expect(a.privateKey).not.toEqual(b.privateKey);
  });
});

describe('randomShortId', () => {
  it('returns lowercase hex of the requested byte length', () => {
    const id = randomShortId(4);
    expect(id).toMatch(/^[0-9a-f]{8}$/);
  });
});

const CONFIG: RealityConfig = {
  address: 'example.com',
  port: 443,
  uuid: '11111111-2222-3333-4444-555555555555',
  dest: 'www.microsoft.com:443',
  serverNames: ['www.microsoft.com'],
  shortIds: ['ab12'],
  privateKey: 'PRIV',
  publicKey: 'PUB',
  fingerprint: 'chrome',
  spiderX: '/',
  flow: 'xtls-rprx-vision',
};

describe('realityClientLink', () => {
  it('builds a parseable vless link with the public REALITY params', () => {
    const link = realityClientLink(CONFIG);
    const parsed = parseLink(link);
    expect(parsed.protocol).toBe('vless');
    expect(parsed.credential).toBe(CONFIG.uuid);
    expect(parsed.port).toBe(443);
    expect(parsed.params.security).toBe('reality');
    expect(parsed.params.pbk).toBe('PUB');
    expect(parsed.params.sid).toBe('ab12');
    expect(parsed.params.sni).toBe('www.microsoft.com');
    expect(parsed.params.flow).toBe('xtls-rprx-vision');
    // The private key must never appear in a client link.
    expect(link).not.toContain('PRIV');
  });
});

describe('realityServerInbound', () => {
  it('produces a vless+reality inbound with the private key', () => {
    const inbound = realityServerInbound(CONFIG) as {
      protocol: string;
      streamSettings: { security: string; realitySettings: { privateKey: string; dest: string } };
    };
    expect(inbound.protocol).toBe('vless');
    expect(inbound.streamSettings.security).toBe('reality');
    expect(inbound.streamSettings.realitySettings.privateKey).toBe('PRIV');
    expect(inbound.streamSettings.realitySettings.dest).toBe('www.microsoft.com:443');
  });
});
