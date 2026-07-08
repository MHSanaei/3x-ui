import { describe, it, expect } from 'vitest';
import {
  bytesToBase64,
  base64ToBytes,
  bytesToBase64Url,
  base64UrlToBytes,
  textToBase64,
  base64ToText,
} from './base64';

describe('base64', () => {
  it('round-trips raw bytes through standard base64', () => {
    const bytes = new Uint8Array([0, 1, 2, 250, 251, 252, 253, 254, 255]);
    expect(base64ToBytes(bytesToBase64(bytes))).toEqual(bytes);
  });

  it('encodes URL-safe base64 without padding or +//', () => {
    const bytes = new Uint8Array([251, 255, 191, 0]);
    const url = bytesToBase64Url(bytes);
    expect(url).not.toMatch(/[+/=]/);
    expect(base64UrlToBytes(url)).toEqual(bytes);
  });

  it('tolerates missing padding when decoding', () => {
    // "M" => 0x33; "TQ" decodes to one byte 0x4d.
    expect(base64ToBytes('TQ')).toEqual(new Uint8Array([0x4d]));
  });

  it('round-trips UTF-8 text (including non-ASCII) for vmess payloads', () => {
    const text = JSON.stringify({ ps: 'سرور تهران', net: 'ws' });
    expect(base64ToText(textToBase64(text))).toBe(text);
  });
});
