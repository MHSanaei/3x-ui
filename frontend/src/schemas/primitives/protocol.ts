import { z } from 'zod';

export const ProtocolSchema = z.enum([
  'vmess',
  'vless',
  'trojan',
  'shadowsocks',
  'wireguard',
  'hysteria',
  'hysteria2',
  'http',
  'mixed',
  'tunnel',
]);
export type Protocol = z.infer<typeof ProtocolSchema>;
