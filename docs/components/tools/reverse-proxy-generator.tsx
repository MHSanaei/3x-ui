'use client';

import { useState } from 'react';
import {
  buildProxyConfig,
  buildCertCommand,
  type ProxyServer,
  type CertTool,
  type ReverseProxyOptions,
} from '@/lib/xray/reverse-proxy';
import { ToolFrame } from './tool-frame';
import { TextField, SelectField } from './shared/fields';
import { OutputBlock } from './shared/output-block';

export function ReverseProxyGenerator() {
  const [server, setServer] = useState<ProxyServer>('nginx');
  const [domain, setDomain] = useState('panel.example.com');
  const [panelPort, setPanelPort] = useState('2053');
  const [panelPath, setPanelPath] = useState('/panel');
  const [certTool, setCertTool] = useState<CertTool>('certbot');

  const options: ReverseProxyOptions = { server, domain, panelPort, panelPath, certTool };

  return (
    <ToolFrame
      title="Reverse-proxy config generator"
      description="Generate an Nginx or Caddy reverse-proxy config (with WebSocket support) and a matching certificate command."
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <SelectField
          label="Server"
          value={server}
          onChange={(v) => setServer(v as ProxyServer)}
          options={['nginx', 'caddy']}
        />
        <TextField label="Domain" value={domain} onChange={setDomain} />
        <TextField
          label="Panel port"
          value={panelPort}
          onChange={setPanelPort}
          inputMode="numeric"
        />
        <TextField label="Panel web base path" value={panelPath} onChange={setPanelPath} />
        {server === 'nginx' ? (
          <SelectField
            label="Certificate tool"
            value={certTool}
            onChange={(v) => setCertTool(v as CertTool)}
            options={['certbot', 'acme.sh']}
          />
        ) : null}
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4">
        <OutputBlock
          label={server === 'nginx' ? 'nginx server block' : 'Caddyfile'}
          value={buildProxyConfig(options)}
        />
        {server === 'nginx' ? (
          <OutputBlock label="Obtain a certificate" value={buildCertCommand(options)} />
        ) : (
          <p className="text-sm text-fd-muted-foreground">
            Caddy obtains and renews TLS certificates automatically — no extra command needed.
          </p>
        )}
      </div>
    </ToolFrame>
  );
}
