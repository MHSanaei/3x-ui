import { useEffect, useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import BasicsTab from '@/pages/xray/basics/BasicsTab';
import RoutingTab from '@/pages/xray/routing/RoutingTab';
import type { SetTemplate, XraySettingsValue } from '@/hooks/useXraySetting';
import { renderWithProviders } from './test-utils';

const SCRIPT_BANNER = 'A routing script is set under Basics, so Xray ignores these rules.';

function renderRoutingRules(routing: Record<string, unknown>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithProviders(
    <QueryClientProvider client={queryClient}>
      <RoutingTab
        templateSettings={{ routing } as unknown as XraySettingsValue}
        setTemplateSettings={vi.fn()}
        inboundTags={[]}
        clientReverseTags={[]}
        isMobile={false}
      />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole('tab', { name: /Routing Rules/ }));
}

// xray-core 26.10.10 routes only through routing.script once it is set, skipping
// every rule, the panel's api rule included.
describe('routing script', () => {
  it('writes the script path from Basics and drops the key once cleared', () => {
    const seen = vi.fn();
    const latest = () => seen.mock.lastCall?.[0] as { routing: Record<string, unknown> };
    function Harness() {
      const [templateSettings, setTemplateSettings] = useState<XraySettingsValue | null>({
        routing: { domainStrategy: 'AsIs', rules: [] },
        outbounds: [],
      } as unknown as XraySettingsValue);
      useEffect(() => seen(templateSettings), [templateSettings]);
      const updateTemplate: SetTemplate = (next) => {
        setTemplateSettings((current) => (typeof next === 'function' ? next(current) : next));
      };
      return (
        <BasicsTab
          templateSettings={templateSettings}
          setTemplateSettings={updateTemplate}
          outboundTestUrl=""
          onChangeOutboundTestUrl={vi.fn()}
          onResetDefault={vi.fn()}
        />
      );
    }

    renderWithProviders(<Harness />);
    const script = screen.getByRole('textbox', { name: 'Routing Script' });
    fireEvent.change(script, { target: { value: 'route.lua' } });
    expect(latest().routing.script).toBe('route.lua');

    fireEvent.change(script, { target: { value: '  ' } });
    expect(latest().routing).not.toHaveProperty('script');
  });

  it('warns above the rules while a script is set', () => {
    renderRoutingRules({ rules: [], script: 'route.lua' });

    expect(screen.getByText(SCRIPT_BANNER)).toBeTruthy();
  });

  it('shows no warning without a script', () => {
    renderRoutingRules({ rules: [] });

    expect(screen.queryByText(SCRIPT_BANNER)).toBeNull();
  });
});
