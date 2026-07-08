'use client';

import { useMemo, useState } from 'react';
import { AlertCircle } from 'lucide-react';
import { parseLink, type ParsedLink } from '@/lib/xray/links';
import { ToolFrame } from './tool-frame';

type Result = { ok: true; data: ParsedLink } | { ok: false; error: string } | null;

export function ShareLinkInspector() {
  const [input, setInput] = useState('');

  const result: Result = useMemo(() => {
    const value = input.trim();
    if (!value) return null;
    try {
      return { ok: true, data: parseLink(value) };
    } catch (e) {
      return { ok: false, error: (e as Error).message };
    }
  }, [input]);

  return (
    <ToolFrame
      title="Share-link inspector"
      description="Paste a vless / vmess / trojan / ss link to decode every parameter. It is parsed entirely in your browser — nothing is sent over the network."
      onReset={input ? () => setInput('') : undefined}
    >
      <textarea
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="vless://uuid@host:443?security=reality&pbk=...#name"
        dir="ltr"
        rows={3}
        spellCheck={false}
        className="w-full resize-y rounded-lg border bg-fd-background px-3 py-2 font-mono text-sm outline-none transition-colors focus-visible:border-fd-primary focus-visible:ring-2 focus-visible:ring-fd-ring/30"
      />

      {result && !result.ok ? (
        <div className="mt-3 flex items-center gap-2 rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-400">
          <AlertCircle className="size-4 shrink-0" aria-hidden />
          <span>{result.error}</span>
        </div>
      ) : null}

      {result && result.ok ? <ResultTable data={result.data} /> : null}
    </ToolFrame>
  );
}

function ResultTable({ data }: { data: ParsedLink }) {
  const rows: [string, string][] = [
    ['Protocol', data.protocol],
    ['Name', data.name],
    ['Address', data.address],
    ['Port', String(data.port)],
    [data.protocol === 'trojan' ? 'Password' : 'ID / credential', data.credential],
    ...Object.entries(data.params),
  ];

  return (
    <div className="mt-3 overflow-hidden rounded-xl border">
      <table className="w-full text-sm">
        <tbody>
          {rows.map(([key, value], i) => (
            <tr key={`${key}-${i}`} className="border-b last:border-b-0">
              <th
                scope="row"
                className="w-1/3 bg-fd-muted/40 px-3 py-2 text-start align-top font-medium text-fd-muted-foreground"
              >
                {key}
              </th>
              <td dir="ltr" className="break-all px-3 py-2 text-start font-mono">
                {value || <span className="text-fd-muted-foreground">—</span>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
