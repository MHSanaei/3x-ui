import { z } from 'zod';

import { WsHeaderMapSchema } from '@/schemas/protocols/stream/ws';

export const XHttpModeSchema = z.enum(['auto', 'packet-up', 'stream-up', 'stream-one']);
export type XHttpMode = z.infer<typeof XHttpModeSchema>;

// xHTTP (SplitHTTPConfig) is xray-core's modern stream-multiplexed transport.
// The field set is large because the schema mirrors what the server-side
// listener reads — plus a few client-only fields (`uplinkHTTPMethod`,
// `headers`) the panel embeds into share-link `extra` blobs even though the
// server ignores them at runtime. Outbound has additional fields (uplinkChunk
// sizes, noGRPCHeader, scMinPostsIntervalMs, xmux, downloadSettings) which
// belong on the outbound class instead, not modeled here.
export const XHttpStreamSettingsSchema = z.object({
  path: z.string().default('/'),
  host: z.string().default(''),
  mode: XHttpModeSchema.default('auto'),
  xPaddingBytes: z.string().default('100-1000'),
  xPaddingObfsMode: z.boolean().default(false),
  xPaddingKey: z.string().default(''),
  xPaddingHeader: z.string().default(''),
  xPaddingPlacement: z.string().default(''),
  xPaddingMethod: z.string().default(''),
  sessionPlacement: z.string().default(''),
  sessionKey: z.string().default(''),
  seqPlacement: z.string().default(''),
  seqKey: z.string().default(''),
  uplinkDataPlacement: z.string().default(''),
  uplinkDataKey: z.string().default(''),
  scMaxEachPostBytes: z.string().default('1000000'),
  noSSEHeader: z.boolean().default(false),
  scMaxBufferedPosts: z.number().int().min(0).default(30),
  scStreamUpServerSecs: z.string().default('20-80'),
  serverMaxHeaderBytes: z.number().int().min(0).default(0),
  uplinkHTTPMethod: z.string().default(''),
  headers: WsHeaderMapSchema.default({}),
});
export type XHttpStreamSettings = z.infer<typeof XHttpStreamSettingsSchema>;
