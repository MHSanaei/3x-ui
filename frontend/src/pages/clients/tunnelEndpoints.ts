import { hostEndpointsFor, type HostEndpoint } from '@/lib/hosts/host-link';
import { preferPublicHost, resolveShareHost } from '@/lib/xray/inbound-link';
import type { InboundOption } from '@/hooks/useClients';
import type { HostRecord } from '@/schemas/api/host';

// One client config per Host the subscription of that format advertises for the
// inbound; `undefined` stands for the inbound's own address when no Host applies.
export function tunnelConfigEndpoints(
  inbound: InboundOption,
  hosts: HostRecord[],
  host: string,
  publicHost: string,
  subType: 'raw' | 'clash' = 'raw',
): (HostEndpoint | undefined)[] {
  const defaultDest = resolveShareHost(
    inbound,
    inbound.nodeAddress ?? '',
    preferPublicHost(host, publicHost),
  );
  const endpoints = hostEndpointsFor(hosts, inbound.id, inbound.port ?? 0, defaultDest, subType);
  return endpoints.length > 0 ? endpoints : [undefined];
}

// A Host group shares one remark across its addresses, so only dest:port is unique.
export function tunnelEndpointLabel(endpoint: HostEndpoint | undefined): string {
  return endpoint ? `${endpoint.dest}:${endpoint.port}` : '';
}
