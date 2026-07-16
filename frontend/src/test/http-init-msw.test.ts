import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { http, HttpResponse } from 'msw';

import { httpRequest, setupHttp } from '@/api/http-init';

import { CSRF_TOKEN } from './msw/handlers';
import { server } from './msw/server';

const ORIGIN = 'http://localhost';

describe('httpRequest against the MSW-mocked network', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { querySelector: () => null });
    window.X_UI_BASE_PATH = ORIGIN;
    setupHttp();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    delete window.X_UI_BASE_PATH;
  });

  it('fetches a CSRF token, then refetches and retries once after a 403', async () => {
    let posts = 0;
    const sentTokens: (string | null)[] = [];
    server.use(
      http.get(`${ORIGIN}/csrf-token`, () => HttpResponse.json({ success: true, obj: CSRF_TOKEN })),
      http.post(`${ORIGIN}/panel/api/test`, ({ request }) => {
        posts += 1;
        sentTokens.push(request.headers.get('X-CSRF-Token'));
        if (posts === 1) return new HttpResponse(null, { status: 403 });
        return HttpResponse.json({ success: true, obj: 'saved' });
      }),
    );

    const res = await httpRequest('POST', '/panel/api/test', { hello: 'world' });

    expect(posts).toBe(2);
    expect(sentTokens).toEqual([CSRF_TOKEN, CSRF_TOKEN]);
    expect(res.status).toBe(200);
    expect(res.data).toEqual({ success: true, obj: 'saved' });
  });

  it('on a 403 refetches a fresh token from the server even when a stale meta tag is present', async () => {
    const STALE = 'stale-meta-token';
    const FRESH = 'fresh-server-token';
    vi.stubGlobal('document', {
      querySelector: (selector: string) =>
        selector === 'meta[name="csrf-token"]' ? { getAttribute: () => STALE } : null,
    });
    setupHttp();

    let posts = 0;
    const sentTokens: (string | null)[] = [];
    server.use(
      http.get(`${ORIGIN}/csrf-token`, () => HttpResponse.json({ success: true, obj: FRESH })),
      http.post(`${ORIGIN}/panel/api/test`, ({ request }) => {
        posts += 1;
        const token = request.headers.get('X-CSRF-Token');
        sentTokens.push(token);
        if (token !== FRESH) return new HttpResponse(null, { status: 403 });
        return HttpResponse.json({ success: true, obj: 'saved' });
      }),
    );

    const res = await httpRequest('POST', '/panel/api/test', { hello: 'world' });

    expect(sentTokens).toEqual([STALE, FRESH]);
    expect(posts).toBe(2);
    expect(res.status).toBe(200);
  });

  it('resolves a safe GET without requesting a CSRF token', async () => {
    let csrfHits = 0;
    server.use(
      http.get(`${ORIGIN}/csrf-token`, () => {
        csrfHits += 1;
        return HttpResponse.json({ success: true, obj: CSRF_TOKEN });
      }),
      http.get(`${ORIGIN}/panel/api/status`, () => HttpResponse.json({ success: true, obj: { up: true } })),
    );

    const res = await httpRequest('GET', '/panel/api/status');

    expect(csrfHits).toBe(0);
    expect(res.status).toBe(200);
    expect(res.data).toEqual({ success: true, obj: { up: true } });
  });
});
