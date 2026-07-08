'use client';

import { useState } from 'react';
import {
  buildUfwCommands,
  buildNftablesRuleset,
  type FirewallOptions,
  type PortProtocol,
  type PortRule,
} from '@/lib/xray/firewall';
import { ToolFrame } from './tool-frame';
import { CheckboxField } from './shared/fields';
import { OutputBlock } from './shared/output-block';

interface Row {
  label: string;
  port: string;
  protocol: PortProtocol;
  enabled: boolean;
}

const DEFAULT_ROWS: Row[] = [
  { label: 'panel', port: '2053', protocol: 'tcp', enabled: true },
  { label: 'subscription', port: '2096', protocol: 'tcp', enabled: false },
  { label: 'inbound (HTTPS)', port: '443', protocol: 'tcp', enabled: true },
  { label: 'inbound (UDP)', port: '443', protocol: 'udp', enabled: false },
];

export function FirewallRulesGenerator() {
  const [allowSsh, setAllowSsh] = useState(true);
  const [sshPort] = useState('22');
  const [rows, setRows] = useState<Row[]>(DEFAULT_ROWS);

  function toggle(index: number, enabled: boolean) {
    setRows((prev) => prev.map((r, i) => (i === index ? { ...r, enabled } : r)));
  }

  const ports: PortRule[] = rows
    .filter((r) => r.enabled && Number(r.port) > 0)
    .map((r) => ({ port: Number(r.port), protocol: r.protocol, label: r.label }));

  const options: FirewallOptions = { ports, allowSsh, sshPort: Number(sshPort) || 22 };

  return (
    <ToolFrame
      title="Firewall rules generator"
      description="Pick the ports to open and copy ready-made ufw and nftables rules."
    >
      <div className="flex flex-col gap-2">
        <CheckboxField
          label={`Allow SSH (port ${sshPort})`}
          checked={allowSsh}
          onChange={setAllowSsh}
        />
        {rows.map((row, i) => (
          <CheckboxField
            key={`${row.label}-${row.protocol}`}
            label={`${row.label} — ${row.port}/${row.protocol}`}
            checked={row.enabled}
            onChange={(c) => toggle(i, c)}
          />
        ))}
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4">
        <OutputBlock label="ufw" value={buildUfwCommands(options)} />
        <OutputBlock label="nftables (/etc/nftables.conf)" value={buildNftablesRuleset(options)} />
      </div>
    </ToolFrame>
  );
}
