import type { ReactNode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

import { setupHttp } from '@/api/http-init';
import { useClients } from '@/hooks/useClients';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil } from '@/utils';

const ORIGIN = 'https://example.invalid';
const BASE = '/audit/panel/api/clients/';

describe('legacy dot-name client recovery', () => {
  const requests: { url: URL; method: string; body: unknown }[] = [];

  beforeAll(() => {
    vi.mocked(HttpUtil.get).mockRestore();
    vi.mocked(HttpUtil.post).mockRestore();
  });

  beforeEach(() => {
    requests.length = 0;
    window.X_UI_BASE_PATH = `${ORIGIN}/audit/`;
    setupHttp();
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: string, init?: RequestInit) => {
        const url = new URL(input);
        const method = init?.method ?? 'GET';
        if (!url.pathname.startsWith(BASE)) {
          const obj = url.pathname.endsWith('/csrf-token') ? 'audit-csrf' : {};
          return Response.json({ success: true, obj });
        }
        if (url.pathname.endsWith('/options') || url.pathname.endsWith('/onlines')) {
          return Response.json({ success: true, obj: [] });
        }
        const body = init?.body ? JSON.parse(String(init.body)) : null;
        requests.push({ url, method, body });
        const [action, segment] = url.pathname.slice(BASE.length).split('/');
        const email =
          url.searchParams.get('email') ?? (segment ? decodeURIComponent(segment) : null);
        if (action === 'get' && email) {
          return Response.json({
            success: true,
            obj: { client: { email, uuid: 'kept-uuid' }, inboundIds: [7] },
          });
        }
        if (['update', 'del'].includes(action) && email) {
          return Response.json({ success: true });
        }
        return Response.json({ success: false, msg: 'Missing client identity' }, { status: 404 });
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    delete window.X_UI_BASE_PATH;
    setupHttp();
  });

  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={makeTestQueryClient()}>{children}</QueryClientProvider>;
  }

  it.each(['.', '..'])('loads the full record and renames a legacy %j client', async (email) => {
    const { result } = renderHook(() => useClients({ list: false }), { wrapper });
    await act(async () => {
      const full = await result.current.hydrate(email);
      expect(full?.client.uuid).toBe('kept-uuid');
      expect(full?.inboundIds).toEqual([7]);
      const msg = await result.current.update(email, {
        email: 'restored-client',
        id: full?.client.uuid,
      });
      expect(msg.success).toBe(true);
    });
    const update = requests.find((r) => r.url.pathname === `${BASE}update`);
    expect(update?.url.searchParams.get('email')).toBe(email);
    expect(update?.body).toEqual({ email: 'restored-client', id: 'kept-uuid' });
  });

  it.each(['.', '..'])(
    'individually deletes legacy %j while preserving keepTraffic',
    async (email) => {
      const { result } = renderHook(() => useClients({ list: false }), { wrapper });
      await act(async () => {
        const msg = await result.current.remove(email, true);
        expect(msg.success).toBe(true);
      });
      const request = requests.find((r) => r.url.pathname === `${BASE}del`);
      expect(request?.method).toBe('POST');
      expect(request?.url.searchParams.get('email')).toBe(email);
      expect(request?.url.searchParams.get('keepTraffic')).toBe('1');
    },
  );

  it.each(['...', '.alice', 'alice.', 'alice.example', 'alice..example'])(
    'keeps the existing path endpoints for %s',
    async (email) => {
      const { result } = renderHook(() => useClients({ list: false }), { wrapper });
      await act(async () => {
        expect((await result.current.hydrate(email))?.client.email).toBe(email);
        expect((await result.current.update(email, { email })).success).toBe(true);
        expect((await result.current.remove(email, true)).success).toBe(true);
      });
      for (const action of ['get', 'update', 'del']) {
        const request = requests.find((r) => r.url.pathname === `${BASE}${action}/${email}`);
        expect(request).toBeDefined();
        expect(request?.url.searchParams.has('email')).toBe(false);
      }
      expect(
        requests
          .find((r) => r.url.pathname === `${BASE}del/${email}`)
          ?.url.searchParams.get('keepTraffic'),
      ).toBe('1');
    },
  );
});
