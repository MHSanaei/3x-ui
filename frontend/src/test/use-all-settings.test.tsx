import type { ReactNode } from 'react';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useAllSettings } from '@/api/queries/useAllSettings';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('useAllSettings', () => {
  it('keeps backend-accepted settings editable when the frontend schema is stricter', async () => {
    const subJsonUserAgentRegex = 'x'.repeat(2_049);
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', { subJsonUserAgentRegex }));
    const queryClient = makeTestQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result } = renderHook(() => useAllSettings(), { wrapper });

    await waitFor(() => expect(result.current.fetched).toBe(true));
    expect(result.current.allSetting.subJsonUserAgentRegex).toBe(subJsonUserAgentRegex);
  });
});
