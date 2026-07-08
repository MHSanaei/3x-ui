'use client';

import { useId, useState } from 'react';
import { buildCurl, buildFetchSnippet, type ApiRequestInput, type HttpMethod } from '@/lib/xray/api-client';
import { ToolFrame } from './tool-frame';
import { TextField, SelectField } from './shared/fields';
import { OutputBlock } from './shared/output-block';

const METHODS: readonly HttpMethod[] = ['GET', 'POST', 'PUT', 'DELETE'];

export function ApiRequestBuilder() {
  const [baseUrl, setBaseUrl] = useState('https://panel.example.com:2053');
  const [token, setToken] = useState('');
  const [path, setPath] = useState('/panel/api/inbounds/list');
  const [method, setMethod] = useState<HttpMethod>('GET');
  const [body, setBody] = useState('');
  const bodyId = useId();

  const showBody = method === 'POST' || method === 'PUT';
  const input: ApiRequestInput = { baseUrl, token: token || '<token>', path, method, body };

  function reset() {
    setBaseUrl('https://panel.example.com:2053');
    setToken('');
    setPath('/panel/api/inbounds/list');
    setMethod('GET');
    setBody('');
  }

  return (
    <ToolFrame
      title="API request builder"
      description="Build an authenticated cURL command or fetch() snippet for any 3x-ui panel API endpoint under /panel/api/*."
      onReset={reset}
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <TextField label="Panel base URL" value={baseUrl} onChange={setBaseUrl} />
        <TextField
          label="API token (Bearer)"
          value={token}
          onChange={setToken}
          placeholder="Settings → Security → API Token"
        />
        <TextField label="Endpoint path" value={path} onChange={setPath} />
        <SelectField
          label="Method"
          value={method}
          onChange={(v) => setMethod(v as HttpMethod)}
          options={METHODS}
        />
      </div>

      {showBody ? (
        <div className="mt-4 flex flex-col gap-1.5">
          <label htmlFor={bodyId} className="text-sm font-medium">
            Request body (JSON)
          </label>
          <textarea
            id={bodyId}
            dir="ltr"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={4}
            placeholder='{"id": 1}'
            className="rounded-lg border bg-fd-background px-3 py-2 font-mono text-sm outline-none transition-colors focus-visible:border-fd-primary focus-visible:ring-2 focus-visible:ring-fd-ring/30"
          />
        </div>
      ) : null}

      <div className="mt-4 grid grid-cols-1 gap-4">
        <OutputBlock label="cURL" value={buildCurl(input)} />
        <OutputBlock label="fetch()" value={buildFetchSnippet(input)} />
      </div>
    </ToolFrame>
  );
}
