import { fireEvent, screen } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import SubscriptionGeneralTab from '@/pages/settings/SubscriptionGeneralTab';
import { renderWithProviders } from './test-utils';

function LocationProbe() {
  const location = useLocation();
  return (
    <output data-testid="location">
      {location.pathname}
      {location.search}
      {location.hash}
    </output>
  );
}

describe('SubscriptionGeneralTab', () => {
  it('keeps the stored subscription port when the field is cleared', () => {
    const updateSetting = vi.fn();

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings#subscription']}>
        <SubscriptionGeneralTab
          allSetting={new AllSetting({ subPort: 2096 })}
          updateSetting={updateSetting}
        />
      </MemoryRouter>,
    );

    const portInput = screen.getByDisplayValue('2096');
    fireEvent.change(portInput, { target: { value: '' } });
    fireEvent.blur(portInput);

    expect(updateSetting).not.toHaveBeenCalled();
    expect((portInput as HTMLInputElement).value).toBe('2096');
  });

  it('forwards typed subscription ports unchanged', () => {
    const updateSetting = vi.fn();

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings#subscription']}>
        <SubscriptionGeneralTab
          allSetting={new AllSetting({ subPort: 2096 })}
          updateSetting={updateSetting}
        />
      </MemoryRouter>,
    );

    fireEvent.change(screen.getByDisplayValue('2096'), { target: { value: '8443' } });

    expect(updateSetting).toHaveBeenCalledWith({ subPort: 8443 });
  });

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

  it('updates the Happ provider gate from the Happ subscription tab', () => {
    const updateSetting = vi.fn();

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings#subscription']}>
        <SubscriptionGeneralTab
          allSetting={new AllSetting({ happLinkEnable: false })}
          updateSetting={updateSetting}
        />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('tab', { name: /Happ/ }));
    expect(
      screen.getByText(
        "When enabled, this client's complete current subscription URL is sent to crypto.happ.su to generate an encrypted happ://crypt5/ subscription link. (Only for Happ)",
      ),
    ).toBeTruthy();
    fireEvent.click(screen.getByRole('switch', { name: 'Enable Happ encrypted link generation' }));

    expect(updateSetting).toHaveBeenCalledWith({ happLinkEnable: true });
  });

  it('opens the Happ tab from the semantic subscription deep link without changing settings', () => {
    const updateSetting = vi.fn();

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings?subscriptionTab=happ#subscription']}>
        <SubscriptionGeneralTab
          allSetting={new AllSetting({ happLinkEnable: false })}
          updateSetting={updateSetting}
        />
        <LocationProbe />
      </MemoryRouter>,
    );

    expect(screen.getByRole('tab', { name: /Happ/ }).getAttribute('aria-selected')).toBe('true');
    expect(screen.getByText('Enable Happ encrypted link generation')).toBeTruthy();
    expect(screen.getByTestId('location').textContent).toBe(
      '/settings?subscriptionTab=happ#subscription',
    );
    expect(updateSetting).not.toHaveBeenCalled();
  });
});
