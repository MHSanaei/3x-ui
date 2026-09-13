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

  it.each([false, true])(
    'updates the Happ link gate from its own tab when stored as %s',
    (enabled) => {
      const updateSetting = vi.fn();

      renderWithProviders(
        <MemoryRouter initialEntries={['/settings#subscription']}>
          <SubscriptionGeneralTab
            allSetting={new AllSetting({ happLinkEnable: enabled })}
            updateSetting={updateSetting}
          />
        </MemoryRouter>,
      );

      fireEvent.click(screen.getByRole('tab', { name: /Happ/ }));
      expect(
        screen.getByRole('tab', { name: /Routing & Rules/ }).getAttribute('aria-selected'),
      ).toBe('true');
      expect(screen.queryByRole('switch', { name: 'Encrypted subscription links' })).toBeNull();
      fireEvent.click(screen.getByRole('tab', { name: /Subscription Links/ }));
      const linkSwitch = screen.getByRole('switch', { name: 'Encrypted subscription links' });
      expect(linkSwitch.getAttribute('aria-checked')).toBe(String(enabled));
      expect(updateSetting).not.toHaveBeenCalled();
      fireEvent.click(linkSwitch);

      expect(updateSetting).toHaveBeenCalledExactlyOnceWith({ happLinkEnable: !enabled });
    },
  );

  it('opens the Happ link tab from the QR settings deep link without enabling generation', () => {
    const updateSetting = vi.fn();

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings?subscriptionTab=happ&happTab=links#subscription']}>
        <SubscriptionGeneralTab
          allSetting={new AllSetting({ happLinkEnable: false })}
          updateSetting={updateSetting}
        />
        <LocationProbe />
      </MemoryRouter>,
    );

    expect(screen.getByRole('tab', { name: /Happ/ }).getAttribute('aria-selected')).toBe('true');
    expect(
      screen.getByRole('tab', { name: /Subscription Links/ }).getAttribute('aria-selected'),
    ).toBe('true');
    expect(
      screen
        .getByRole('switch', { name: 'Encrypted subscription links' })
        .getAttribute('aria-checked'),
    ).toBe('false');
    expect(screen.getByTestId('location').textContent).toBe(
      '/settings?subscriptionTab=happ&happTab=links#subscription',
    );
    expect(updateSetting).not.toHaveBeenCalled();
  });

  it.each(['', '&happTab=unknown'])(
    'keeps the routing default for a general Happ deep link %s',
    (query) => {
      const updateSetting = vi.fn();

      renderWithProviders(
        <MemoryRouter initialEntries={['/settings?subscriptionTab=happ' + query + '#subscription']}>
          <SubscriptionGeneralTab allSetting={new AllSetting()} updateSetting={updateSetting} />
        </MemoryRouter>,
      );

      expect(screen.getByRole('tab', { name: /Happ/ }).getAttribute('aria-selected')).toBe('true');
      expect(
        screen.getByRole('tab', { name: /Routing & Rules/ }).getAttribute('aria-selected'),
      ).toBe('true');
      expect(screen.queryByRole('switch', { name: 'Encrypted subscription links' })).toBeNull();
      expect(updateSetting).not.toHaveBeenCalled();
    },
  );
});
