import { describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';

import ObservatorySettingsTab from '@/pages/xray/balancers/ObservatorySettingsTab';
import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { renderWithProviders } from './test-utils';

function renderTab(templateSettings: XraySettingsValue) {
  renderWithProviders(
    <ObservatorySettingsTab
      templateSettings={templateSettings}
      mutate={vi.fn()}
    />,
  );
}

describe('ObservatorySettingsTab', () => {
  it('shows one burst settings panel and warns for legacy mixed observers', () => {
    renderTab({
      routing: {
        balancers: [{ tag: 'll', selector: ['proxy-a'], strategy: { type: 'leastLoad' } }],
      },
      observatory: { subjectSelector: ['stale-regular'] },
      burstObservatory: { subjectSelector: ['proxy-a'] },
    } as unknown as XraySettingsValue);

    expect(screen.getByText(/This config contains both Observatory and Burst Observatory/)).toBeTruthy();
    expect(document.querySelector('.ant-segmented')).toBeFalsy();
    expect(screen.getByText('Probe Destination')).toBeTruthy();
    expect(screen.queryByText('Probe URL')).toBeFalsy();
  });

  it('shows regular observatory settings for mixed legacy configs that normalize back to leastPing only', () => {
    renderTab({
      routing: {
        balancers: [{ tag: 'lp', selector: ['proxy-b'], strategy: { type: 'leastPing' } }],
      },
      observatory: { subjectSelector: ['proxy-b'] },
      burstObservatory: { subjectSelector: ['stale-burst'] },
    } as unknown as XraySettingsValue);

    expect(screen.getByText(/This config contains both Observatory and Burst Observatory/)).toBeTruthy();
    expect(document.querySelector('.ant-segmented')).toBeFalsy();
    expect(screen.getByText('Probe URL')).toBeTruthy();
    expect(screen.queryByText('Probe Destination')).toBeFalsy();
  });
});
