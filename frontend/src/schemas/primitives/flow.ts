import { z } from 'zod';

export const FlowSchema = z.enum([
  '',
  'xtls-rprx-vision',
  'xtls-rprx-vision-udp443',
]);
export type Flow = z.infer<typeof FlowSchema>;
