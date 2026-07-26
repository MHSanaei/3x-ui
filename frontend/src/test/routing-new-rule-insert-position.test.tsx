import { describe, it, expect, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import RoutingTab from '@/pages/xray/routing/RoutingTab';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

import { renderWithProviders } from './test-utils';

function settingsWithApiAndBlockRules(): XraySettingsValue {
  return {
    routing: {
      rules: [
        { type: 'field', inboundTag: ['api'], outboundTag: 'api', enabled: true },
        { type: 'field', ip: ['ext:geoip_RU.dat:ru'], outboundTag: 'blocked', enabled: true },
        { type: 'field', protocol: ['bittorrent'], outboundTag: 'blocked', enabled: true },
      ],
    },
  } as unknown as XraySettingsValue;
}

// Rules match top-to-bottom, first hit wins. A brand-new rule used to be
// appended at the end, where a pre-existing broader/catch-all rule (e.g. a
// block rule with no inboundTag restriction, like the ones here) silently
// shadows it forever -- the rule looks saved and enabled but never actually
// fires. See RoutingTab.tsx onRuleConfirm.
describe('RoutingTab new-rule insert position', () => {
  it('inserts a newly created rule right after the pinned api rule, not at the end', () => {
    const setTemplateSettings = vi.fn();
    const initial = settingsWithApiAndBlockRules();

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderWithProviders(
      <QueryClientProvider client={queryClient}>
        <RoutingTab
          templateSettings={initial}
          setTemplateSettings={setTemplateSettings}
          inboundTags={['awg-tag']}
          clientReverseTags={[]}
          isMobile={false}
        />
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole('tab', { name: /Routing Rules/ }));
    fireEvent.click(screen.getByRole('button', { name: /Routing Rules/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    expect(setTemplateSettings).toHaveBeenCalledTimes(1);
    const updater = setTemplateSettings.mock.calls[0][0] as (prev: XraySettingsValue) => XraySettingsValue;
    const next = updater(initial);
    const rules = (next.routing as { rules: Array<{ inboundTag?: string[]; outboundTag?: string }> }).rules;

    expect(rules.length).toBe(4);
    expect(rules[0].outboundTag).toBe('api');
    // The two pre-existing block rules must have been pushed down, not the
    // new rule appended after them.
    expect(rules[1].outboundTag).not.toBe('blocked');
    expect(rules[2].outboundTag).toBe('blocked');
    expect(rules[3].outboundTag).toBe('blocked');
  });
});
