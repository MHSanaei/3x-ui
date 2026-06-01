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
