import { describe, it, expect, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import RoutingTab from '@/pages/xray/routing/RoutingTab';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

import { renderWithProviders } from './test-utils';

function settingsWithHiddenLoopback(): XraySettingsValue {
  return {
    routing: {
      rules: [
        { type: 'field', outboundTag: 'a', enabled: true },
        { type: 'field', inboundTag: ['_bl_bal1'], outboundTag: 'b', enabled: true },
        { type: 'field', outboundTag: 'c', enabled: true },
      ],
    },
  } as unknown as XraySettingsValue;
}

describe('RoutingTab hidden-loopback index mapping', () => {
  it('toggles the visible rule, not the hidden loopback rule that precedes it', () => {
    const setTemplateSettings = vi.fn();
    const initial = settingsWithHiddenLoopback();

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderWithProviders(
      <QueryClientProvider client={queryClient}>
        <RoutingTab
          templateSettings={initial}
          setTemplateSettings={setTemplateSettings}
          inboundTags={[]}
          clientReverseTags={[]}
          isMobile={false}
        />
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole('tab', { name: /Routing Rules/ }));

    const switches = document.querySelectorAll('.routing-table .ant-switch');
    expect(switches.length).toBe(2);

    fireEvent.click(switches[1]);

    expect(setTemplateSettings).toHaveBeenCalledTimes(1);
    const updater = setTemplateSettings.mock.calls[0][0] as (prev: XraySettingsValue) => XraySettingsValue;
    const next = updater(initial);
    const rules = (next.routing as { rules: Array<{ enabled?: boolean }> }).rules;

    expect(rules[2].enabled).toBe(false);
    expect(rules[1].enabled).toBe(true);
  });
});
