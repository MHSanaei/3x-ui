import { fireEvent, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import SubscriptionGeneralTab from '@/pages/settings/SubscriptionGeneralTab';
import { renderWithProviders } from './test-utils';

describe('calendar expiry presentation setting', () => {
  it('is off by default and updates only the presentation option', () => {
    const updateSetting = vi.fn();

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings#subscription']}>
        <SubscriptionGeneralTab allSetting={new AllSetting()} updateSetting={updateSetting} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('tab', { name: /Information/ }));
    const toggle = screen.getByRole('switch', { name: 'Month-end subscription expiry display' });
    expect(toggle.getAttribute('aria-checked')).toBe('false');
    expect(updateSetting).not.toHaveBeenCalled();
    fireEvent.click(toggle);

    expect(updateSetting).toHaveBeenCalledExactlyOnceWith({ subCalendarExpireInclusive: true });
  });
});
