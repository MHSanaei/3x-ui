import { afterEach, describe, expect, it, vi } from 'vitest';

import { PromiseUtil } from '@/utils';

const fast = { delay: 0, interval: 1, timeout: 200 };

function answer(status: number, type: ResponseType = 'basic'): Response {
  return { status, type } as Response;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('PromiseUtil.waitForPanel (#7190)', () => {
  it('keeps polling through refused connections and proxy 502s until the panel answers', async () => {
    vi.stubGlobal('window', {
      location: { href: 'https://panel.test/panel/settings', origin: 'https://panel.test' },
    });
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce(answer(502))
      .mockResolvedValueOnce(answer(200));
    vi.stubGlobal('fetch', fetchMock);

    await expect(PromiseUtil.waitForPanel('https://panel.test/panel/settings', fast)).resolves.toBe(
      true,
    );
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ mode: 'same-origin', cache: 'no-store' });
  });

  it('gives up after the timeout so the caller still navigates', async () => {
    vi.stubGlobal('window', {
      location: { href: 'https://panel.test/', origin: 'https://panel.test' },
    });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(answer(502)));

    await expect(
      PromiseUtil.waitForPanel('https://panel.test/', { ...fast, timeout: 20 }),
    ).resolves.toBe(false);
  });

  it('probes a new port or protocol with no-cors and accepts the opaque answer', async () => {
    vi.stubGlobal('window', {
      location: {
        href: 'http://203.0.113.5:2053/panel/settings',
        origin: 'http://203.0.113.5:2053',
      },
    });
    const fetchMock = vi.fn().mockResolvedValue(answer(0, 'opaque'));
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      PromiseUtil.waitForPanel('https://203.0.113.5:8443/panel/settings', fast),
    ).resolves.toBe(true);
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ mode: 'no-cors' });
  });

  it('probes the current origin when an https page would open an http target (mixed content)', async () => {
    vi.stubGlobal('window', {
      location: {
        href: 'https://panel.test/panel/settings',
        origin: 'https://panel.test',
        protocol: 'https:',
      },
    });
    const fetchMock = vi.fn().mockResolvedValue(answer(200));
    vi.stubGlobal('fetch', fetchMock);

    await expect(PromiseUtil.waitForPanel('http://panel.test/panel/settings', fast)).resolves.toBe(
      true,
    );
    expect(String(fetchMock.mock.calls[0][0])).toBe('https://panel.test/panel/settings');
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ mode: 'same-origin' });
  });
});
