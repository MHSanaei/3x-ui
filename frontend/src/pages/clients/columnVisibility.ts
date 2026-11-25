import { useCallback, useState } from 'react';
import type { ReactNode } from 'react';
import type { ColumnsType } from 'antd/es/table';

const HIDDEN_COLUMNS_KEY = 'clientsHiddenColumns';

// The client (email) column is deliberately absent: hiding it would leave rows
// with nothing to tell them apart.
const HIDEABLE_CLIENT_COLUMNS = [
  'actions',
  'enable',
  'online',
  'group',
  'inboundIds',
  'traffic',
  'speed',
  'remaining',
  'expiryTime',
] as const;

export type HideableClientColumn = (typeof HIDEABLE_CLIENT_COLUMNS)[number];

function isHideableClientColumn(key: unknown): key is HideableClientColumn {
  return (HIDEABLE_CLIENT_COLUMNS as readonly unknown[]).includes(key);
}

export interface ClientColumnOption {
  key: HideableClientColumn;
  label: ReactNode;
}

// A column already hidden for its own reason (Group with no groups defined) is
// not offered: toggling it would do nothing visible.
export function clientColumnOptions<T>(columns: ColumnsType<T>): ClientColumnOption[] {
  return columns.flatMap((column) =>
    isHideableClientColumn(column.key) && !column.hidden && typeof column.title !== 'function'
      ? [{ key: column.key, label: column.title }]
      : [],
  );
}

export function hideClientColumns<T>(
  columns: ColumnsType<T>,
  hidden: readonly HideableClientColumn[],
): ColumnsType<T> {
  return columns.map((column) =>
    isHideableClientColumn(column.key) && hidden.includes(column.key)
      ? { ...column, hidden: true }
      : column,
  );
}

function readHiddenClientColumns(): HideableClientColumn[] {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(HIDDEN_COLUMNS_KEY) || '[]');
    return Array.isArray(raw) ? raw.filter(isHideableClientColumn) : [];
  } catch {
    return [];
  }
}

export function useHiddenClientColumns() {
  const [hidden, setHiddenState] = useState<HideableClientColumn[]>(readHiddenClientColumns);
  const setHidden = useCallback((next: HideableClientColumn[]) => {
    setHiddenState(next);
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify(next));
  }, []);
  return [hidden, setHidden] as const;
}
