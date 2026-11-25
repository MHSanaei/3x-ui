export interface ClientTrafficWindowInfo {
  id: number;
  email: string;
  at: number;
  up: number;
  down: number;
  reason: string;
}

export interface ClientNodeUsageInfo {
  nodeId: number;
  nodeName: string;
  up: number;
  down: number;
}

export interface ClientChangeInfo {
  id: number;
  email: string;
  at: number;
  field: string;
  oldValue: string;
  newValue: string;
  source: string;
}

export interface ClientUsageHistoryInfo {
  email: string;
  lifetimeUp: number;
  lifetimeDown: number;
  currentUp: number;
  currentDown: number;
  windows: ClientTrafficWindowInfo[];
  nodes: ClientNodeUsageInfo[];
  changes: ClientChangeInfo[];
}

const num = (value: unknown, fallback = 0): number =>
  typeof value === 'number' && Number.isFinite(value) ? value : fallback;

// The backend omits empty collections; the modal always wants real arrays.
export function normalizeClientUsageHistory(raw: unknown): ClientUsageHistoryInfo {
  const obj = (raw ?? {}) as Record<string, unknown>;
  const rows = (value: unknown): Array<Record<string, unknown>> =>
    Array.isArray(value)
      ? value.filter(
          (entry): entry is Record<string, unknown> => !!entry && typeof entry === 'object',
        )
      : [];
  return {
    email: typeof obj.email === 'string' ? obj.email : '',
    lifetimeUp: num(obj.lifetimeUp),
    lifetimeDown: num(obj.lifetimeDown),
    currentUp: num(obj.currentUp),
    currentDown: num(obj.currentDown),
    windows: rows(obj.windows).map((w) => ({
      id: num(w.id),
      email: String(w.email ?? ''),
      at: num(w.at),
      up: num(w.up),
      down: num(w.down),
      reason: String(w.reason ?? ''),
    })),
    nodes: rows(obj.nodes).map((n) => ({
      nodeId: num(n.nodeId),
      nodeName: String(n.nodeName ?? ''),
      up: num(n.up),
      down: num(n.down),
    })),
    changes: rows(obj.changes).map((c) => ({
      id: num(c.id),
      email: String(c.email ?? ''),
      at: num(c.at),
      field: String(c.field ?? ''),
      oldValue: String(c.oldValue ?? ''),
      newValue: String(c.newValue ?? ''),
      source: String(c.source ?? ''),
    })),
  };
}
