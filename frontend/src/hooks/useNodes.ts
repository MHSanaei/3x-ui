import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { HttpUtil } from '@/utils';

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

interface ApiMsg<T = unknown> {
  success?: boolean;
  msg?: string;
  obj?: T;
}

interface NodeTotals {
  total: number;
  online: number;
  offline: number;
  avgLatency: number;
  inbounds: number;
  clients: number;
  onlineClients: number;
  depleted: number;
}

export function useNodes() {
  const [nodes, setNodes] = useState<NodeRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetched, setFetched] = useState(false);
  const fetchedRef = useRef(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.get('/panel/api/nodes/list') as ApiMsg<NodeRecord[]>;
      if (msg?.success) {
        setNodes(Array.isArray(msg.obj) ? msg.obj : []);
      }
      fetchedRef.current = true;
      setFetched(true);
    } finally {
      setLoading(false);
    }
  }, []);

  const applyNodesEvent = useCallback((payload: unknown) => {
    if (Array.isArray(payload)) {
      setNodes(payload as NodeRecord[]);
      if (!fetchedRef.current) {
        fetchedRef.current = true;
        setFetched(true);
      }
    }
  }, []);

  const create = useCallback(async (payload: Partial<NodeRecord>) => {
    const msg = await HttpUtil.post('/panel/api/nodes/add', payload) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const update = useCallback(async (id: number, payload: Partial<NodeRecord>) => {
    const msg = await HttpUtil.post(`/panel/api/nodes/update/${id}`, payload) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const remove = useCallback(async (id: number) => {
    const msg = await HttpUtil.post(`/panel/api/nodes/del/${id}`) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const setEnable = useCallback(async (id: number, enable: boolean) => {
    const msg = await HttpUtil.post(`/panel/api/nodes/setEnable/${id}`, { enable }) as ApiMsg;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

  const testConnection = useCallback(async (payload: Partial<NodeRecord>) => {
    return await HttpUtil.post('/panel/api/nodes/test', payload) as ApiMsg<{
      status: string;
      latencyMs?: number;
      xrayVersion?: string;
      error?: string;
    }>;
  }, []);

  const probe = useCallback(async (id: number) => {
    const msg = await HttpUtil.post(`/panel/api/nodes/probe/${id}`) as ApiMsg<{
      status: string;
      latencyMs?: number;
      error?: string;
    }>;
    if (msg?.success) await refresh();
    return msg;
  }, [refresh]);

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

  useEffect(() => {
     
    refresh();
  }, [refresh]);

  return {
    nodes,
    loading,
    fetched,
    totals,
    refresh,
    applyNodesEvent,
    create,
    update,
    remove,
    setEnable,
    testConnection,
    probe,
  };
}
