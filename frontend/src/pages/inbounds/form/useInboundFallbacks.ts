import { useRef, useState } from 'react';

import { HttpUtil } from '@/utils';
import type { FallbackRow } from '@/schemas/forms/inbound-form';
import { coerceInboundJsonField, type DBInbound } from '@/models/dbinbound';

// Fallback rows for VLESS/Trojan TLS inbounds: state + the load/save/derive
// and add/update/remove/move handlers, plus the eligible-child option list.
// Lifted out of InboundFormModal so the modal body stays focused on layout.
export function useInboundFallbacks(dbInbound: DBInbound | null, dbInbounds: DBInbound[]) {
  const fallbackKeyRef = useRef(0);
  const [fallbacks, setFallbacks] = useState<FallbackRow[]>([]);

  const fallbackChildOptions = (dbInbounds || [])
    .filter((ib) => ib.id !== dbInbound?.id)
    .map((ib) => ({
      label: `${ib.remark || `#${ib.id}`} · ${ib.protocol}:${ib.port}`,
      value: ib.id,
    }));

  const loadFallbacks = async (masterId: number | null) => {
    if (!masterId) {
      setFallbacks([]);
      return;
    }
    const msg = await HttpUtil.get(`/panel/api/inbounds/${masterId}/fallbacks`);
    if (!msg?.success || !Array.isArray(msg.obj)) {
      setFallbacks([]);
      return;
    }
    setFallbacks(
      (msg.obj as {
        childId: number;
        name?: string;
        alpn?: string;
        path?: string;
        dest?: string;
        xver?: number;
      }[])
        .map((r) => ({
          rowKey: `fb-${++fallbackKeyRef.current}`,
          childId: r.childId,
          name: r.name || '',
          alpn: r.alpn || '',
          path: r.path || '',
          dest: r.dest || '',
          xver: r.xver || 0,
        })),
    );
  };

  const saveFallbacks = async (masterId: number) => {
    if (!masterId) return true;
    const payload = {
      fallbacks: fallbacks.filter((c) => c.childId).map((c, i) => ({
        childId: c.childId,
        name: c.name,
        alpn: c.alpn,
        path: c.path,
        dest: c.dest,
        xver: Number(c.xver) || 0,
        sortOrder: i,
      })),
    };
    const msg = await HttpUtil.post(
      `/panel/api/inbounds/${masterId}/fallbacks`,
      payload,
      { headers: { 'Content-Type': 'application/json' } },
    );
    return !!msg?.success;
  };

  // Derive a fallback row's SNI / ALPN / Path / xver from a child
  // inbound's streamSettings — what the legacy panel auto-filled when an
  // operator wired a fallback target. SNI/ALPN come straight off the
  // child's TLS block; path depends on the child's transport (ws/grpc
  // /httpupgrade carry an explicit path; tcp/kcp/xhttp have no path of
  // their own). xver stays 0 unless the child explicitly opts in via
  // PROXY-protocol sockopt.
  const deriveFallbackDefaults = (childId: number): Partial<FallbackRow> => {
    const child = (dbInbounds || []).find((ib) => ib.id === childId);
    if (!child) return {};
    const stream = coerceInboundJsonField(child.streamSettings);
    const tls = (stream.tlsSettings as Record<string, unknown> | undefined) ?? {};
    const network = typeof stream.network === 'string' ? stream.network : '';
    const sni = typeof tls.serverName === 'string' ? tls.serverName : '';
    const alpnArr = Array.isArray(tls.alpn) ? tls.alpn : [];
    const alpn = alpnArr.filter((v) => typeof v === 'string').join(',');
    let path = '';
    if (network === 'ws') {
      const ws = (stream.wsSettings as Record<string, unknown> | undefined) ?? {};
      if (typeof ws.path === 'string') path = ws.path;
    } else if (network === 'grpc') {
      const grpc = (stream.grpcSettings as Record<string, unknown> | undefined) ?? {};
      if (typeof grpc.serviceName === 'string') path = grpc.serviceName;
    } else if (network === 'httpupgrade') {
      const hu = (stream.httpupgradeSettings as Record<string, unknown> | undefined) ?? {};
      if (typeof hu.path === 'string') path = hu.path;
    } else if (network === 'xhttp') {
      const xh = (stream.xhttpSettings as Record<string, unknown> | undefined) ?? {};
      if (typeof xh.path === 'string') path = xh.path;
    }
    return { name: sni, alpn, path, xver: 0 };
  };

  const addFallback = () => {
    setFallbacks((prev) => [...prev, {
      rowKey: `fb-${++fallbackKeyRef.current}`,
      childId: null,
      name: '',
      alpn: '',
      path: '',
      dest: '',
      xver: 0,
    }]);
  };

  const updateFallback = (rowKey: string, patch: Partial<FallbackRow>) => {
    setFallbacks((prev) => prev.map((r) => {
      if (r.rowKey !== rowKey) return r;
      // When the picker selects a new child inbound and the row hasn't
      // been hand-edited yet (sni/alpn/path/dest all blank, xver = 0),
      // pull the SNI/ALPN/Path defaults off that child. Operators who
      // intentionally typed values keep them — we only fill the empties.
      if (typeof patch.childId === 'number' && patch.childId !== r.childId) {
        const isPristine = !r.name && !r.alpn && !r.path && !r.dest && r.xver === 0;
        if (isPristine) return { ...r, ...patch, ...deriveFallbackDefaults(patch.childId) };
      }
      return { ...r, ...patch };
    }));
  };

  const removeFallback = (idx: number) => {
    setFallbacks((prev) => prev.filter((_, i) => i !== idx));
  };

  // Move a fallback row up/down by swapping adjacent indices. The order
  // is persisted via the fallback row's sortOrder (rebuilt by index on
  // save), so reordering survives reloads.
  const moveFallback = (idx: number, direction: -1 | 1) => {
    setFallbacks((prev) => {
      const target = idx + direction;
      if (target < 0 || target >= prev.length) return prev;
      const next = prev.slice();
      [next[idx], next[target]] = [next[target], next[idx]];
      return next;
    });
  };

  // One-shot: add a fresh fallback row for every eligible inbound (i.e.
  // every option in fallbackChildOptions) that is not already wired up.
  // Convenient for operators who want catch-all routing to every host
  // they manage on the panel.
  const addAllFallbacks = () => {
    setFallbacks((prev) => {
      const alreadyHave = new Set(prev.map((r) => r.childId));
      const additions = fallbackChildOptions
        .filter((opt) => !alreadyHave.has(opt.value))
        .map<FallbackRow>((opt) => {
          const derived = deriveFallbackDefaults(opt.value);
          return {
            rowKey: `fb-${++fallbackKeyRef.current}`,
            childId: opt.value,
            name: derived.name ?? '',
            alpn: derived.alpn ?? '',
            path: derived.path ?? '',
            dest: '',
            xver: derived.xver ?? 0,
          };
        });
      if (additions.length === 0) return prev;
      return [...prev, ...additions];
    });
  };

  return {
    fallbacks,
    fallbackChildOptions,
    loadFallbacks,
    saveFallbacks,
    addFallback,
    updateFallback,
    removeFallback,
    moveFallback,
    addAllFallbacks,
  };
}
