import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import BasicsTab from '@/pages/xray/basics/BasicsTab';
import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { renderWithProviders } from './test-utils';

function renderBasics(outbounds: Record<string, unknown>[]) {
  renderWithProviders(
    <BasicsTab
      templateSettings={{ outbounds } as unknown as XraySettingsValue}
      setTemplateSettings={vi.fn()}
      outboundTestUrl=""
      onChangeOutboundTestUrl={vi.fn()}
      onResetDefault={vi.fn()}
    />,
  );
  return {
    strategy: screen.getByRole('combobox', { name: 'Freedom Protocol Strategy' }),
    happyEyeballs: screen.getByRole('switch', { name: 'Freedom Happy Eyeballs (IPv4/IPv6)' }),
  };
}

// Both setters drop the edit when a non-freedom outbound holds "direct", so
// the controls must say so instead of snapping back silently.
describe('BasicsTab with the direct tag held by a foreign outbound', () => {
  it('disables the freedom controls', () => {
    const { strategy, happyEyeballs } = renderBasics([
      { protocol: 'socks', tag: 'direct', settings: { servers: [] } },
    ]);

    expect(strategy).toHaveProperty('disabled', true);
    expect(happyEyeballs).toHaveProperty('disabled', true);
  });

  it('keeps them enabled when freedom holds the tag, whatever its spelling', () => {
    const { strategy, happyEyeballs } = renderBasics([
      { protocol: 'Freedom', tag: 'direct', settings: {} },
    ]);

    expect(strategy).toHaveProperty('disabled', false);
    expect(happyEyeballs).toHaveProperty('disabled', false);
  });
});
