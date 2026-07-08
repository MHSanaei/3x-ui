'use client';

import { useState } from 'react';
import {
  buildSubscriptionUrls,
  buildShareLinks,
  buildBase64Subscription,
  buildJsonSubscription,
  type SubClient,
  type SubUrlInput,
} from '@/lib/xray/subscription';
import type { Network, Security } from '@/lib/xray/outbounds';
import { ToolFrame } from './tool-frame';
import { TextField, SelectField, CheckboxField } from './shared/fields';
import { OutputBlock } from './shared/output-block';

type ClientProtocol = 'vless' | 'vmess' | 'trojan' | 'ss';

interface ClientRow {
  protocol: ClientProtocol;
  remark: string;
  address: string;
  port: string;
  credential: string; // id (vless/vmess) or password (trojan/ss)
  method: string; // ss
  network: Network;
  security: Security;
  sni: string;
}

const PROTOCOLS: readonly ClientProtocol[] = ['vless', 'vmess', 'trojan', 'ss'];
const NETWORKS: readonly Network[] = ['tcp', 'kcp', 'ws', 'grpc', 'httpupgrade', 'xhttp'];
const SECURITIES: readonly Security[] = ['none', 'tls', 'reality'];

const addBtn =
  'inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground';

const DEFAULT_CLIENTS: ClientRow[] = [
  {
    protocol: 'vless',
    remark: 'HK-01',
    address: 'a.example.com',
    port: '443',
    credential: '11111111-2222-3333-4444-555555555555',
    method: '',
    network: 'tcp',
    security: 'reality',
    sni: 'www.microsoft.com',
  },
];

function toClient(r: ClientRow): SubClient {
  const isUuid = r.protocol === 'vless' || r.protocol === 'vmess';
  return {
    protocol: r.protocol,
    remark: r.remark,
    address: r.address,
    port: Number(r.port),
    id: isUuid ? r.credential : undefined,
    password: isUuid ? undefined : r.credential,
    method: r.protocol === 'ss' ? r.method : undefined,
    network: r.network,
    security: r.security,
    sni: r.sni || undefined,
  };
}

export function SubscriptionBuilder() {
  const [scheme, setScheme] = useState<'http' | 'https'>('https');
  const [host, setHost] = useState('sub.example.com');
  const [port, setPort] = useState('2096');
  const [subPath, setSubPath] = useState('/sub/');
  const [jsonPath, setJsonPath] = useState('/json/');
  const [subId, setSubId] = useState('user-1');
  const [behindProxy, setBehindProxy] = useState(false);
  const [clients, setClients] = useState<ClientRow[]>(DEFAULT_CLIENTS);

  function patch(i: number, p: Partial<ClientRow>) {
    setClients((prev) => prev.map((c, j) => (i === j ? { ...c, ...p } : c)));
  }

  const urlInput: SubUrlInput = { scheme, host, port: Number(port), subPath, jsonPath, subId, behindProxy };
  const urls = buildSubscriptionUrls(urlInput);
  const subClients = clients.filter((c) => c.address.trim()).map(toClient);

  function reset() {
    setScheme('https');
    setHost('sub.example.com');
    setPort('2096');
    setSubPath('/sub/');
    setJsonPath('/json/');
    setSubId('user-1');
    setBehindProxy(false);
    setClients(DEFAULT_CLIENTS);
  }

  return (
    <ToolFrame
      title="Subscription & sub-JSON builder"
      description="Build the subscription URLs and preview both body formats — the Base64 link list and the JSON (Xray-json) config."
      onReset={reset}
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <SelectField
          label="Scheme"
          value={scheme}
          onChange={(v) => setScheme(v as 'http' | 'https')}
          options={['https', 'http']}
        />
        <TextField label="Host" value={host} onChange={setHost} />
        <TextField label="Port" value={port} onChange={setPort} inputMode="numeric" />
        <TextField label="Sub ID" value={subId} onChange={setSubId} />
        <TextField label="Sub path" value={subPath} onChange={setSubPath} />
        <TextField label="JSON path" value={jsonPath} onChange={setJsonPath} />
        <CheckboxField
          label="Behind a reverse proxy (omit the port)"
          checked={behindProxy}
          onChange={setBehindProxy}
        />
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4">
        <OutputBlock label="Base64 subscription URL" value={urls.base64} qr />
        <OutputBlock label="JSON subscription URL" value={urls.json} />
      </div>

      <div className="mt-5 flex items-center justify-between">
        <h4 className="text-sm font-semibold">Clients in this subscription</h4>
        <button
          type="button"
          className={addBtn}
          onClick={() =>
            setClients((p) => [
              ...p,
              {
                protocol: 'vless',
                remark: '',
                address: '',
                port: '443',
                credential: '',
                method: '',
                network: 'tcp',
                security: 'reality',
                sni: '',
              },
            ])
          }
        >
          Add client
        </button>
      </div>
      <div className="mt-2 flex flex-col gap-3">
        {clients.map((c, i) => (
          <div key={i} className="rounded-xl border p-3">
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <SelectField
                label="Protocol"
                value={c.protocol}
                onChange={(v) => patch(i, { protocol: v as ClientProtocol })}
                options={PROTOCOLS}
              />
              <TextField label="Remark" value={c.remark} onChange={(v) => patch(i, { remark: v })} />
              <TextField label="Address" value={c.address} onChange={(v) => patch(i, { address: v })} />
              <TextField label="Port" value={c.port} onChange={(v) => patch(i, { port: v })} inputMode="numeric" />
              <TextField
                label={c.protocol === 'vless' || c.protocol === 'vmess' ? 'UUID (id)' : 'Password'}
                value={c.credential}
                onChange={(v) => patch(i, { credential: v })}
              />
              {c.protocol === 'ss' ? (
                <TextField label="Method" value={c.method} onChange={(v) => patch(i, { method: v })} />
              ) : null}
              <SelectField
                label="Transport"
                value={c.network}
                onChange={(v) => patch(i, { network: v as Network })}
                options={NETWORKS}
              />
              <SelectField
                label="Security"
                value={c.security}
                onChange={(v) => patch(i, { security: v as Security })}
                options={SECURITIES}
              />
              {c.security !== 'none' ? (
                <TextField label="SNI" value={c.sni} onChange={(v) => patch(i, { sni: v })} />
              ) : null}
            </div>
            <div className="mt-2 flex justify-end">
              <button
                type="button"
                className={addBtn}
                onClick={() => setClients((p) => p.filter((_, j) => j !== i))}
              >
                Remove
              </button>
            </div>
          </div>
        ))}
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4">
        <OutputBlock label="Subscription links (decoded body)" value={buildShareLinks(subClients).join('\n')} />
        <OutputBlock label="Base64 body" value={buildBase64Subscription(subClients)} />
        <OutputBlock label="JSON subscription (preview)" value={buildJsonSubscription(subClients)} />
      </div>
    </ToolFrame>
  );
}
