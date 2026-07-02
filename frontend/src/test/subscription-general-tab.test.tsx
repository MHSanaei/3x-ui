import { fireEvent, screen } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import SubscriptionGeneralTab from '@/pages/settings/SubscriptionGeneralTab';
import { renderWithProviders } from './test-utils';

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{location.pathname}{location.hash}</output>;
}

describe('SubscriptionGeneralTab', () => {
  it('uses router navigation to open subscription format settings', () => {
    const allSetting = new AllSetting({ subClashEnable: true });

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings#subscription']}>
        <SubscriptionGeneralTab allSetting={allSetting} updateSetting={vi.fn()} />
        <LocationProbe />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Open Sub Formats' }));

    expect(screen.getByTestId('location').textContent).toBe('/settings#subscription-formats');
  });
});
