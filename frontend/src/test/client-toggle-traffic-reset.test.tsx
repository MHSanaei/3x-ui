import type { ReactNode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useClients } from '@/hooks/useClients';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('client enable toggle', () => {
  it.each([false, true])(
    'preserves the hydrated traffic reset cycle when enable=%s',
    async (enable) => {
      const email = 'scheduled@example.com';
      vi.spyOn(HttpUtil, 'get').mockResolvedValue(
        new Msg(true, '', {
          client: { email, enable: !enable, trafficReset: 'monthly', trafficResetDay: 15 },
          inboundIds: [],
        }),
      );
      const post = vi
        .spyOn(HttpUtil, 'post')
        .mockImplementation(
          async (url: string) =>
            new Msg(true, '', url.includes('/setting/defaultSettings') ? {} : null),
        );
      const queryClient = makeTestQueryClient();
      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );
      const { result } = renderHook(() => useClients({ list: false }), { wrapper });

      await act(async () => {
        await result.current.setEnable(
          { email, trafficReset: 'never', trafficResetDay: 1 },
          enable,
        );
      });

      expect(HttpUtil.get).toHaveBeenCalledWith('/panel/api/clients/get/scheduled%40example.com');
      expect(post).toHaveBeenCalledWith(
        '/panel/api/clients/update/scheduled%40example.com',
        expect.objectContaining({ email, enable, trafficReset: 'monthly', trafficResetDay: 15 }),
        { headers: { 'Content-Type': 'application/json' } },
      );
    },
  );
});
