import { describe, it, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import OutboundsTab from '@/pages/xray/outbounds/OutboundsTab';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

import { renderWithProviders } from './test-utils';

// The core lowercases the id, so a "VMess" row is a vmess outbound: its stream
// tags must follow the same rule its address does.
function settingsWithCapitalisedProtocol(): XraySettingsValue {
  return {
    outbounds: [
      {
        tag: 'proxy-a',
        protocol: 'VMess',
        settings: { vnext: [{ address: 'a.example.com', port: 443 }] },
        streamSettings: { network: 'ws', security: 'tls' },
      },
    ],
  } as unknown as XraySettingsValue;
}

function renderTab(settings: XraySettingsValue) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithProviders(
    <QueryClientProvider client={queryClient}>
      <OutboundsTab
        templateSettings={settings}
        setTemplateSettings={vi.fn()}
        outboundsTraffic={[]}
        outboundTestStates={{}}
        subscriptionTestStates={{}}
        testingAll={false}
        inboundTags={[]}
        isMobile={false}
        onResetTraffic={vi.fn()}
        onTest={vi.fn()}
        onTestSubscription={vi.fn()}
        onTestAll={vi.fn()}
        onShowWarp={vi.fn()}
        onShowNord={vi.fn()}
        onShowPia={vi.fn()}
      />
    </QueryClientProvider>,
  );
}

describe('OutboundsTab row for a case-variant protocol id', () => {
  it('renders the stream tags and the address of a "VMess" row', () => {
    renderTab(settingsWithCapitalisedProtocol());

    const row = document.querySelector('.ant-table-tbody tr.ant-table-row');
    const tags = Array.from(row?.querySelectorAll('.protocol-line .ant-tag') ?? []).map(
      (el) => el.textContent,
    );
    expect(tags).toEqual(['VMess', 'ws', 'tls']);
    expect(row?.textContent).toContain('a.example.com:443');
  });
});
