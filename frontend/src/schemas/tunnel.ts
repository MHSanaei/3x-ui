import { z } from 'zod';
import {
  ConfigSchema,
  PortSchema,
  TunnelViewSchema,
  TunnelMutationSchema,
  CapabilitiesSchema,
  TunnelTokenSchema,
} from '@/generated/zod';

const port = z.number().int().min(1).max(65535);
export const TunnelPortSchema = PortSchema.extend({
  listen: port,
  target: port,
  protocol: z.enum(['tcp', 'udp']),
});
export const TunnelConfigSchema = ConfigSchema.extend({
  role: z.enum(['client', 'server']),
  carrier: z.enum(['auto', 'icmp', 'tcp', 'udp', 'tls', 'wss']),
  endpoint: z
    .string()
    .max(253)
    .refine((v) => !/[\s|#"']/.test(v)),
  ports: z.array(TunnelPortSchema).min(1).max(128),
  mtu: z.number().int().min(0).max(16384),
  sessions: z.number().int().min(0).max(16),
  conns: z.number().int().min(0).max(64),
  memoryMB: z.number().int().min(0).max(65536),
});
const TunnelFormObject = TunnelMutationSchema.extend({
  name: z.string().trim().min(1).max(128),
  nodeId: z.number().int().positive().nullable(),
  config: TunnelConfigSchema,
});
export const TunnelFormSchema = TunnelFormObject.superRefine((v, ctx) => {
  if (!v.token && !v.config.endpoint)
    ctx.addIssue({
      code: 'custom',
      path: ['config', 'endpoint'],
      message: 'pages.tunnels.endpoint',
    });
});
export type TunnelFormValues = z.infer<typeof TunnelFormSchema>;
export type TunnelRecord = z.infer<typeof TunnelViewSchema>;
export { TunnelViewSchema, CapabilitiesSchema, TunnelTokenSchema };

export function defaultTunnel(): TunnelFormValues {
  return {
    name: '',
    nodeId: null,
    revision: 0,
    enabled: false,
    secret: '',
    token: '',
    generateKey: true,
    config: {
      role: 'server',
      carrier: 'tcp',
      endpoint: '',
      port: 4443,
      reverse: false,
      reversePeer: '',
      bind: '0.0.0.0',
      listen: '0.0.0.0',
      ports: [{ listen: 8443, target: 443, protocol: 'tcp' }],
      candidates: [],
      sni: '',
      path: '/wat',
      tlsPin: '',
      tlsCert: '',
      tlsKey: '',
      mtu: 0,
      sessions: 0,
      conns: 0,
      fec: false,
      memoryMB: 0,
      icmpType: 'echo',
    },
  };
}
