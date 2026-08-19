import type { ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useAllSettings } from '@/api/queries/useAllSettings';
import { keys } from '@/api/queryKeys';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('useAllSettings', () => {
  it('accepts legacy overlength regex settings without logging a response validation warning', async () => {
    const subJsonUserAgentRegex = 'x'.repeat(2_049);
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', { subJsonUserAgentRegex }));
    const warning = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result } = renderHook(() => useAllSettings(), { wrapper });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    expect(result.current.allSetting.subJsonUserAgentRegex).toBe(subJsonUserAgentRegex);
    expect(warning).not.toHaveBeenCalled();
  });

  it('keeps an edited setting when a refetch returns older server data', async () => {
    const values = [{ webPort: 2053 }, { webPort: 2054 }];
    let index = 0;
    vi.spyOn(HttpUtil, 'post').mockImplementation(async () => new Msg(true, '', values[index++]));
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useAllSettings(), { wrapper });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    act(() => result.current.updateSetting({ webPort: 3000 }));
    await queryClient.invalidateQueries({ queryKey: keys.settings.all() });

    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledTimes(2));
    expect(result.current.allSetting.webPort).toBe(3000);
    expect(result.current.saveDisabled).toBe(false);
  });

  it('hydrates redacted secrets after a successful save', async () => {
    let fetchCount = 0;
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (url) => {
      if (url === '/panel/api/setting/all') {
        fetchCount += 1;
        return new Msg(
          true,
          '',
          fetchCount === 1 ? { hasTgBotToken: false } : { hasTgBotToken: true, tgBotToken: '' },
        );
      }
      return new Msg(true, '');
    });
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useAllSettings(), { wrapper });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    act(() => result.current.updateSetting({ tgBotToken: 'secret' }));
    await act(async () => {
      await result.current.saveAll();
    });

    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledTimes(3));
    expect(result.current.allSetting.tgBotToken).toBe('');
    expect(result.current.allSetting.hasTgBotToken).toBe(true);
    expect(result.current.saveDisabled).toBe(true);
  });

  it('establishes a saved baseline for a full-payload security save', async () => {
    let fetchCount = 0;
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (url) => {
      if (url === '/panel/api/setting/all') {
        fetchCount += 1;
        return new Msg(
          true,
          '',
          fetchCount === 1 ? { hasTgBotToken: false } : { hasTgBotToken: true, tgBotToken: '' },
        );
      }
      return new Msg(true, '');
    });
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useAllSettings(), { wrapper });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    act(() => result.current.updateSetting({ tgBotToken: 'secret' }));
    await act(async () => {
      await result.current.savePayload({
        ...result.current.allSetting,
        twoFactorEnable: false,
        twoFactorToken: '',
      });
    });

    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledTimes(3));
    expect(result.current.allSetting.tgBotToken).toBe('');
    expect(result.current.allSetting.hasTgBotToken).toBe(true);
    expect(result.current.saveDisabled).toBe(true);
  });
});
