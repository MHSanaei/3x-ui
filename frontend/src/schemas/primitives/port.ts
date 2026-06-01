import { z } from 'zod';

export const PortSchema = z.number().int().min(1).max(65535);
export type Port = z.infer<typeof PortSchema>;

export const InboundPortSchema = z.number().int().min(0).max(65535);
export type InboundPort = z.infer<typeof InboundPortSchema>;
