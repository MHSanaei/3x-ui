import { useEffect, useRef, useState } from 'react';
import { Button, Input, Space } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';

import { InputAddon } from '@/components/ui';

// Reusable header-map editor. Handles the two wire shapes Xray uses for
// HTTP-style header maps:
//
//   v1:   { 'Content-Type': 'application/json',  'X-Custom': 'value' }
//         Used by WS / HTTPUpgrade / Hysteria masquerade. One value per
//         name.
//
//   v2:   { 'Accept':       ['text/html', 'application/json'],
//           'X-Forwarded':  ['1.2.3.4'] }
//         Used by TCP HTTP camouflage request/response. Each header can
//         repeat (RFC 7230 §3.2.2).
//
// Internal state is always the flat list-of-rows shape regardless of
// mode. Conversion to/from the wire shape happens at the value/onChange
// boundary so consumers can bind straight to a Form.Item without any
// extra transforms on their side.

export type HeaderMapMode = 'v1' | 'v2';

export type HeaderMapValue =
  | Record<string, string>
  | Record<string, string[]>
  | undefined;

interface HeaderRow {
  name: string;
  value: string;
}

interface HeaderMapEditorProps {
  mode: HeaderMapMode;
  value?: HeaderMapValue;
  onChange?: (next: Record<string, string> | Record<string, string[]>) => void;
}

function mapToRows(value: HeaderMapValue): HeaderRow[] {
  if (!value || typeof value !== 'object') return [];
  const out: HeaderRow[] = [];
  for (const [name, raw] of Object.entries(value)) {
    if (Array.isArray(raw)) {
      for (const v of raw) {
        out.push({ name, value: typeof v === 'string' ? v : String(v) });
      }
    } else if (typeof raw === 'string') {
      out.push({ name, value: raw });
    }
  }
  return out;
}

function rowsToMap(rows: HeaderRow[], mode: HeaderMapMode): Record<string, string> | Record<string, string[]> {
  if (mode === 'v1') {
    const map: Record<string, string> = {};
    for (const r of rows) {
      if (!r.name) continue;
      map[r.name] = r.value ?? '';
    }
    return map;
  }
  const map: Record<string, string[]> = {};
  for (const r of rows) {
    if (!r.name) continue;
    const list = map[r.name] ?? [];
    list.push(r.value ?? '');
    map[r.name] = list;
  }
  return map;
}

export default function HeaderMapEditor({ mode, value, onChange }: HeaderMapEditorProps) {
  // Local state holds rows including blanks. Without it, addRow() would
  // append a {name:'', value:''} that rowsToMap immediately filters out
  // before reaching the form, so the new row would never reach UI. The
  // form-bound map only sees rows with non-empty names; blank rows live
  // here until the user fills them in.
  const [rows, setRows] = useState<HeaderRow[]>(() => mapToRows(value));
  const lastEmittedRef = useRef<string>(JSON.stringify(rowsToMap(rows, mode)));

  // Re-sync local rows when the form value changes from outside (modal
  // re-open with edit data, JSON tab edits, etc.) but not when it's our
  // own emission echoing back.
  useEffect(() => {
    const incoming = JSON.stringify(value ?? {});
    if (incoming === lastEmittedRef.current) return;
    setRows(mapToRows(value));
    lastEmittedRef.current = incoming;
  }, [value]);

  function commit(next: HeaderRow[]) {
    setRows(next);
    const map = rowsToMap(next, mode);
    lastEmittedRef.current = JSON.stringify(map);
    onChange?.(map);
  }

  function setRow(index: number, patch: Partial<HeaderRow>) {
    const next = rows.slice();
    next[index] = { ...next[index], ...patch };
    commit(next);
  }

  function addRow() {
    commit([...rows, { name: '', value: '' }]);
  }

  function removeRow(index: number) {
    const next = rows.slice();
    next.splice(index, 1);
    commit(next);
  }

  return (
    <>
      {rows.map((row, idx) => (
        <Space.Compact key={idx} block className="mb-8">
          <InputAddon>{`${idx + 1}`}</InputAddon>
          <Input
            value={row.name}
            placeholder="Name"
            onChange={(e) => setRow(idx, { name: e.target.value })}
          />
          <Input
            value={row.value}
            placeholder="Value"
            onChange={(e) => setRow(idx, { value: e.target.value })}
          />
          <Button icon={<MinusOutlined />} onClick={() => removeRow(idx)} />
        </Space.Compact>
      ))}
      <Button size="small" type="primary" icon={<PlusOutlined />} onClick={addRow}>
        Add
      </Button>
    </>
  );
}
