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

  // connect-src 'self' rejects the probe like a refused connection, so polling another
  // origin could only run to the timeout: 90 s of spinner before a panel that is already up.
  it('does not poll a target on another origin, which the page CSP forbids it to reach', async () => {
    vi.stubGlobal('window', {
      location: {
        href: 'http://203.0.113.5:2053/panel/settings',
        origin: 'http://203.0.113.5:2053',
      },
    });
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      PromiseUtil.waitForPanel('https://203.0.113.5:2053/panel/settings', fast),
    ).resolves.toBe(false);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
