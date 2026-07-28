import { describe, it, expect, vi } from 'vitest';

import BasicsTab from '@/pages/xray/basics/BasicsTab';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

import { renderWithProviders } from './test-utils';

function settingsWithMalformedHappyEyeballs(): XraySettingsValue {
  return {
    outbounds: [
      {
        protocol: 'freedom',
        tag: 'direct',
        streamSettings: { sockopt: { happyEyeballs: { tryDelayMs: 'fast' } } },
      },
    ],
  } as unknown as XraySettingsValue;
}

describe('BasicsTab malformed happyEyeballs', () => {
  it('renders instead of white-screening on a wrong-typed happyEyeballs value', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() =>
      renderWithProviders(
        <BasicsTab
          templateSettings={settingsWithMalformedHappyEyeballs()}
          setTemplateSettings={vi.fn()}
          outboundTestUrl=""
          onChangeOutboundTestUrl={vi.fn()}
          onResetDefault={vi.fn()}
        />,
      ),
    ).not.toThrow();
    errorSpy.mockRestore();
  });
});
