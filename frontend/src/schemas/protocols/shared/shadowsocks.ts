import { z } from 'zod';

export const SSMethodSchema = z.enum([
  'aes-256-gcm',
  'chacha20-poly1305',
  'chacha20-ietf-poly1305',
  'xchacha20-ietf-poly1305',
  '2022-blake3-aes-128-gcm',
  '2022-blake3-aes-256-gcm',
  '2022-blake3-chacha20-poly1305',
]);
export type SSMethod = z.infer<typeof SSMethodSchema>;

// xray-core refuses a 2022 chacha20 inbound with users: the multi-user identity
// header is AES-only, and every panel inbound carries users.
export const SSInboundMethodSchema = SSMethodSchema.exclude(['2022-blake3-chacha20-poly1305']);
