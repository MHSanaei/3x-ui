import { DnsQueryStrategySchema } from '@/schemas/dns';
import type { DnsServerValue } from './DnsServerModal';

export const STRATEGIES = DnsQueryStrategySchema.options;
export const DEFAULT_FAKEDNS = () => ({ ipPool: '198.18.0.0/15', poolSize: 65535 });

export function addrFor(server: DnsServerValue): string {
  return typeof server === 'string' ? server : server?.address || '';
}

export function domainsFor(server: DnsServerValue): string {
  return typeof server === 'object' && server !== null ? (server.domains || []).join(',') : '';
}

export function expectedIPsFor(server: DnsServerValue): string {
  if (typeof server !== 'object' || !server) return '';
  const list = server.expectedIPs || server.expectIPs || [];
  return Array.isArray(list) ? list.join(',') : '';
}
