export interface ClientFilters {
  buckets: string[];
  protocols: string[];
  inboundIds: number[];
  // Node ids to filter by; 0 is the "local panel" sentinel (inbounds with
  // no nodeId). Mapped onto inbound ids client-side — see ClientsPage.
  nodeIds: number[];
  groups: string[];
  expiryFrom?: number;
  expiryTo?: number;
  usageFromGB?: number;
  usageToGB?: number;
  autoRenew: '' | 'on' | 'off';
  hasTgId: '' | 'yes' | 'no';
  hasComment: '' | 'yes' | 'no';
}

export function emptyFilters(): ClientFilters {
  return {
    buckets: [],
    protocols: [],
    inboundIds: [],
    nodeIds: [],
    groups: [],
    autoRenew: '',
    hasTgId: '',
    hasComment: '',
  };
}

export function activeFilterCount(f: ClientFilters): number {
  let n = 0;
  if (f.buckets.length) n++;
  if (f.protocols.length) n++;
  if (f.inboundIds.length) n++;
  if (f.nodeIds.length) n++;
  if (f.groups.length) n++;
  if (f.expiryFrom || f.expiryTo) n++;
  if (f.usageFromGB || f.usageToGB) n++;
  if (f.autoRenew) n++;
  if (f.hasTgId) n++;
  if (f.hasComment) n++;
  return n;
}
