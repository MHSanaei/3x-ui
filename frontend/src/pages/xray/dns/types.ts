import type { DnsObject } from '@/schemas/dns';
import type { DnsServerValue } from './DnsServerModal';

export type DnsConfig = Omit<DnsObject, 'servers'> & { servers?: DnsServerValue[] };

export interface HostRow {
  domain: string;
  values: string[];
}

export interface FakednsRow {
  ipPool: string;
  poolSize: number;
}
