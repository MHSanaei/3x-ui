import { z } from 'zod';

export const SlimInboundSchema = z.object({
  id: z.number(),
  protocol: z.string(),
}).loose();

export const SlimInboundListSchema = z.array(SlimInboundSchema);

export const InboundDetailSchema = z.object({
  id: z.number(),
  protocol: z.string(),
}).loose();

export const LastOnlineMapSchema = z.record(z.string(), z.number());

export const InboundFormSchema = z.object({
  remark: z.string(),
  enable: z.boolean(),
  port: z
    .number({ error: 'pages.inbounds.toasts.portRequired' })
    .int()
    .min(1, 'pages.inbounds.toasts.portRange')
    .max(65535, 'pages.inbounds.toasts.portRange'),
  listen: z.string(),
  protocol: z.string().min(1, 'pages.inbounds.toasts.protocolRequired'),
});

export type SlimInbound = z.infer<typeof SlimInboundSchema>;
export type InboundDetail = z.infer<typeof InboundDetailSchema>;
export type LastOnlineMap = z.infer<typeof LastOnlineMapSchema>;
export type InboundFormValues = z.infer<typeof InboundFormSchema>;
