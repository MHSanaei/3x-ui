import { z } from 'zod';

// Xray's V2-style header map: { Host: ['example.com', ...], ... }. Each
// header name maps to a string[] because HTTP allows repeated headers
// (Accept, Cookie, etc.). The panel renders these as a flat name/value
// table internally and flattens to this map on save via toV2Headers.
export const V2HeaderMapSchema = z.record(z.string(), z.array(z.string()));
export type V2HeaderMap = z.infer<typeof V2HeaderMapSchema>;

export const TcpRequestSchema = z.object({
  version: z.string().default('1.1'),
  method: z.string().default('GET'),
  path: z.array(z.string()).min(1).default(['/']),
  headers: V2HeaderMapSchema.default({}),
});
export type TcpRequest = z.infer<typeof TcpRequestSchema>;

export const TcpResponseSchema = z.object({
  version: z.string().default('1.1'),
  status: z.string().default('200'),
  reason: z.string().default('OK'),
  headers: V2HeaderMapSchema.default({}),
});
export type TcpResponse = z.infer<typeof TcpResponseSchema>;

// TCP stream `header` is the obfuscation header. type='none' (the wire
// representation just omits `header` entirely) or type='http' (HTTP-1.1
// camouflage with request/response sub-objects).
export const TcpHeaderHttpSchema = z.object({
  type: z.literal('http'),
  request: TcpRequestSchema.optional(),
  response: TcpResponseSchema.optional(),
});
export const TcpHeaderNoneSchema = z.object({ type: z.literal('none') });
export const TcpHeaderSchema = z.discriminatedUnion('type', [
  TcpHeaderNoneSchema,
  TcpHeaderHttpSchema,
]);
export type TcpHeader = z.infer<typeof TcpHeaderSchema>;

export const TcpStreamSettingsSchema = z.object({
  acceptProxyProtocol: z.boolean().default(false),
  header: TcpHeaderSchema.optional(),
});
export type TcpStreamSettings = z.infer<typeof TcpStreamSettingsSchema>;
