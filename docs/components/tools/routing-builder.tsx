'use client';

import { useState } from 'react';
import {
  buildRoutingJson,
  type DomainStrategy,
  type RoutingInput,
  type RuleNetwork,
  type Strategy,
} from '@/lib/xray/routing';
import { ToolFrame } from './tool-frame';
import { TextField, SelectField } from './shared/fields';
import { OutputBlock } from './shared/output-block';

interface BalancerRow {
  tag: string;
  selector: string;
  strategy: Strategy;
  fallbackTag: string;
}

interface RuleRow {
  domain: string;
  ip: string;
  port: string;
  network: string;
  inboundTag: string;
  targetKind: 'outbound' | 'balancer';
  targetTag: string;
}

const STRATEGIES: readonly Strategy[] = ['random', 'roundRobin', 'leastPing', 'leastLoad'];
const NETWORKS = ['any', 'tcp', 'udp', 'tcp,udp'];
const TARGET_KINDS = ['outbound', 'balancer'];
const DOMAIN_STRATEGIES: readonly DomainStrategy[] = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'];

const DEFAULT_BALANCERS: BalancerRow[] = [
  { tag: 'balancer', selector: 'proxy', strategy: 'leastPing', fallbackTag: '' },
];
const DEFAULT_RULES: RuleRow[] = [
  { domain: 'geosite:category-ads-all', ip: '', port: '', network: 'any', inboundTag: '', targetKind: 'outbound', targetTag: 'block' },
  { domain: '', ip: 'geoip:private', port: '', network: 'any', inboundTag: '', targetKind: 'outbound', targetTag: 'direct' },
];

function list(s: string): string[] {
  return s
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean);
}

const addBtn =
  'inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground';

export function RoutingBuilder() {
  const [domainStrategy, setDomainStrategy] = useState<DomainStrategy>('IPIfNonMatch');
  const [balancers, setBalancers] = useState<BalancerRow[]>(DEFAULT_BALANCERS);
  const [rules, setRules] = useState<RuleRow[]>(DEFAULT_RULES);

  function patchBalancer(i: number, patch: Partial<BalancerRow>) {
    setBalancers((prev) => prev.map((b, j) => (i === j ? { ...b, ...patch } : b)));
  }
  function patchRule(i: number, patch: Partial<RuleRow>) {
    setRules((prev) => prev.map((r, j) => (i === j ? { ...r, ...patch } : r)));
  }

  const input: RoutingInput = {
    domainStrategy,
    balancers: balancers
      .filter((b) => b.tag.trim())
      .map((b) => ({
        tag: b.tag.trim(),
        selector: list(b.selector),
        strategy: b.strategy,
        fallbackTag: b.fallbackTag.trim() || undefined,
      })),
    rules: rules
      .filter((r) => r.targetTag.trim())
      .map((r) => ({
        domain: list(r.domain),
        ip: list(r.ip),
        port: r.port.trim() || undefined,
        network: r.network === 'any' ? undefined : (r.network as RuleNetwork),
        inboundTag: list(r.inboundTag),
        target: { kind: r.targetKind, tag: r.targetTag.trim() },
      })),
  };

  function reset() {
    setDomainStrategy('IPIfNonMatch');
    setBalancers(DEFAULT_BALANCERS);
    setRules(DEFAULT_RULES);
  }

  return (
    <ToolFrame
      title="Balancer & routing builder"
      description="Compose Xray balancers and routing rules, then copy the routing block (with a matching observatory for leastPing/leastLoad)."
      onReset={reset}
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <SelectField
          label="Domain strategy"
          value={domainStrategy}
          onChange={(v) => setDomainStrategy(v as DomainStrategy)}
          options={DOMAIN_STRATEGIES}
        />
      </div>

      <div className="mt-5 flex items-center justify-between">
        <h4 className="text-sm font-semibold">Balancers</h4>
        <button
          type="button"
          className={addBtn}
          onClick={() =>
            setBalancers((p) => [...p, { tag: '', selector: '', strategy: 'random', fallbackTag: '' }])
          }
        >
          Add balancer
        </button>
      </div>
      <div className="mt-2 flex flex-col gap-3">
        {balancers.map((b, i) => (
          <div key={i} className="rounded-xl border p-3">
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <TextField label="Tag" value={b.tag} onChange={(v) => patchBalancer(i, { tag: v })} />
              <TextField
                label="Selector (comma-separated prefixes)"
                value={b.selector}
                onChange={(v) => patchBalancer(i, { selector: v })}
              />
              <SelectField
                label="Strategy"
                value={b.strategy}
                onChange={(v) => patchBalancer(i, { strategy: v as Strategy })}
                options={STRATEGIES}
              />
              <TextField
                label="Fallback tag"
                value={b.fallbackTag}
                onChange={(v) => patchBalancer(i, { fallbackTag: v })}
                placeholder="optional"
              />
            </div>
            <div className="mt-2 flex justify-end">
              <button
                type="button"
                className={addBtn}
                onClick={() => setBalancers((p) => p.filter((_, j) => j !== i))}
              >
                Remove
              </button>
            </div>
          </div>
        ))}
      </div>

      <div className="mt-5 flex items-center justify-between">
        <h4 className="text-sm font-semibold">Rules</h4>
        <button
          type="button"
          className={addBtn}
          onClick={() =>
            setRules((p) => [
              ...p,
              { domain: '', ip: '', port: '', network: 'any', inboundTag: '', targetKind: 'outbound', targetTag: '' },
            ])
          }
        >
          Add rule
        </button>
      </div>
      <div className="mt-2 flex flex-col gap-3">
        {rules.map((r, i) => (
          <div key={i} className="rounded-xl border p-3">
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <TextField label="Domain (comma)" value={r.domain} onChange={(v) => patchRule(i, { domain: v })} placeholder="geosite:google, example.com" />
              <TextField label="IP (comma)" value={r.ip} onChange={(v) => patchRule(i, { ip: v })} placeholder="geoip:cn, 1.1.1.1" />
              <TextField label="Port" value={r.port} onChange={(v) => patchRule(i, { port: v })} placeholder="443 or 1000-2000" />
              <SelectField label="Network" value={r.network} onChange={(v) => patchRule(i, { network: v })} options={NETWORKS} />
              <TextField label="Inbound tag (comma)" value={r.inboundTag} onChange={(v) => patchRule(i, { inboundTag: v })} placeholder="optional" />
              <SelectField label="Target kind" value={r.targetKind} onChange={(v) => patchRule(i, { targetKind: v as 'outbound' | 'balancer' })} options={TARGET_KINDS} />
              <TextField label="Target tag" value={r.targetTag} onChange={(v) => patchRule(i, { targetTag: v })} />
            </div>
            <div className="mt-2 flex justify-end">
              <button
                type="button"
                className={addBtn}
                onClick={() => setRules((p) => p.filter((_, j) => j !== i))}
              >
                Remove
              </button>
            </div>
          </div>
        ))}
      </div>

      <div className="mt-4">
        <OutputBlock label="Routing block (Xray JSON)" value={buildRoutingJson(input)} />
      </div>
    </ToolFrame>
  );
}
