import { z } from 'zod';

export const PortSchema = z.number().int().min(1).max(65535);
export type Port = z.infer<typeof PortSchema>;
