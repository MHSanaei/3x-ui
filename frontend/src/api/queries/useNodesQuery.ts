import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

export interface NodeRecord {
  id: number;
  name?: string;
  remark?: string;
  scheme?: string;
  address?: string;
  port?: number;
  basePath?: string;
  apiToken?: string;
  enable?: boolean;
  status?: 'online' | 'offline' | string;
  latencyMs?: number;
  cpuPct?: number;
  memPct?: number;
  xrayVersion?: string;
  panelVersion?: string;
  uptimeSecs?: number;
  inboundCount?: number;
  clientCount?: number;
  onlineCount?: number;
  depletedCount?: number;
  lastHeartbeat?: number;
  lastError?: string;
  allowPrivateAddress?: boolean;
  [key: string]: unknown;
}

export interface NodeTotals {
  total: number;
  online: number;
  offline: number;
  avgLatency: number;
  inbounds: number;
  clients: number;
  onlineClients: number;
  depleted: number;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  msg?: string;
  obj?: T;
}

async function fetchNodes(): Promise<NodeRecord[]> {
  const msg = await HttpUtil.get('/panel/api/nodes/list', undefined, { silent: true }) as ApiMsg<NodeRecord[]>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch nodes');
  return Array.isArray(msg.obj) ? msg.obj : [];
}

export function useNodesQuery() {
  const query = useQuery({
    queryKey: keys.nodes.list(),
    queryFn: fetchNodes,
  });

  const nodes = useMemo(() => query.data ?? [], [query.data]);

  const totals = useMemo<NodeTotals>(() => {
    let online = 0;
    let offline = 0;
    let latencySum = 0;
    let latencyCount = 0;
    let inbounds = 0;
    let clients = 0;
    let onlineClients = 0;
    let depleted = 0;
    for (const n of nodes) {
      inbounds += n.inboundCount || 0;
      clients += n.clientCount || 0;
      onlineClients += n.onlineCount || 0;
      depleted += n.depletedCount || 0;
      if (!n.enable) continue;
      if (n.status === 'online') {
        online += 1;
        if (n.latencyMs && n.latencyMs > 0) {
          latencySum += n.latencyMs;
          latencyCount += 1;
        }
      } else if (n.status === 'offline') {
        offline += 1;
      }
    }
    return {
      total: nodes.length,
      online,
      offline,
      avgLatency: latencyCount > 0 ? Math.round(latencySum / latencyCount) : 0,
      inbounds,
      clients,
      onlineClients,
      depleted,
    };
  }, [nodes]);

  return {
    nodes,
    totals,
    loading: query.isFetching,
    fetched: query.data !== undefined,
  };
}
