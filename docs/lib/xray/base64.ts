// Browser- and Node-safe base64 / base64url helpers used by the in-browser
// config tools. No Node `Buffer` so the same code runs in the browser and in
// vitest (Node). `btoa`/`atob` and `TextEncoder`/`TextDecoder` are available in
// both environments.

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
  return btoa(binary);
}

/** Decode standard or URL-safe base64, tolerating missing padding. */
export function base64ToBytes(b64: string): Uint8Array {
  const normalized = b64.replace(/-/g, '+').replace(/_/g, '/').replace(/\s/g, '');
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=');
  const binary = atob(padded);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) out[i] = binary.charCodeAt(i);
  return out;
}

/** Encode bytes as URL-safe base64 with no padding (xray's key format). */
export function bytesToBase64Url(bytes: Uint8Array): string {
  return bytesToBase64(bytes).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function base64UrlToBytes(s: string): Uint8Array {
  return base64ToBytes(s);
}

const encoder = new TextEncoder();
const decoder = new TextDecoder();

/** UTF-8 text → standard base64 (used by vmess:// links). */
export function textToBase64(text: string): string {
  return bytesToBase64(encoder.encode(text));
}

/** standard/URL-safe base64 → UTF-8 text. */
export function base64ToText(b64: string): string {
  return decoder.decode(base64ToBytes(b64));
}
