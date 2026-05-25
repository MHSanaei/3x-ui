import { z } from 'zod';

// gRPC stream is the lightest transport — three booleans/strings, no
// header obfuscation. `multiMode` enables multi-stream gRPC (multiple
// concurrent streams over one connection).
export const GrpcStreamSettingsSchema = z.object({
  serviceName: z.string().default(''),
  authority: z.string().default(''),
  multiMode: z.boolean().default(false),
});
export type GrpcStreamSettings = z.infer<typeof GrpcStreamSettingsSchema>;
