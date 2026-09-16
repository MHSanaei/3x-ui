import { useState } from 'react';
import { fireEvent, screen } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import SubscriptionGeneralTab from '@/pages/settings/SubscriptionGeneralTab';
import { chooseSelectOption, renderWithProviders } from './test-utils';

function ProfileSettingsHarness({ initial }: { initial?: unknown }) {
  const [allSetting, setAllSetting] = useState(() => new AllSetting(initial));

  return (
    <>
      <SubscriptionGeneralTab
        allSetting={allSetting}
        updateSetting={(patch) =>
          setAllSetting((current) => new AllSetting({ ...current, ...patch }))
        }
      />
      <output data-testid="profile-settings">
        {allSetting.subProfileMode}|{allSetting.subProfileUrl}
      </output>
    </>
  );
}

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
  it('switches profile modes without losing the custom URL and warns only for the built-in page', () => {
    const storedUrl = 'https://example.com/profile/{{SUB_ID}}';
    const editedUrl = 'https://example.com/account/{{SUB_ID}}';
    const warning =
      'This page exposes subscription URLs and node configurations, including for Happ encrypted subscriptions.';

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings#subscription']}>
        <ProfileSettingsHarness initial={{ subProfileMode: 'none', subProfileUrl: storedUrl }} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('tab', { name: /Profile/ }));
    expect(screen.getByRole('combobox', { name: 'Profile page' })).toBeTruthy();
    expect(screen.getByTestId('profile-settings').textContent).toBe(`none|${storedUrl}`);
    expect(screen.queryByDisplayValue(storedUrl)).toBeNull();
    expect(screen.queryByText(warning)).toBeNull();

    chooseSelectOption('sub-profile-mode', 'Built-in subscription page');
    expect(screen.getByTestId('profile-settings').textContent).toBe(`builtin|${storedUrl}`);
    expect(screen.getByRole('alert').textContent).toContain(warning);
    expect(screen.queryByDisplayValue(storedUrl)).toBeNull();

    chooseSelectOption('sub-profile-mode', 'Custom website');
    expect(screen.queryByText(warning)).toBeNull();
    fireEvent.change(screen.getByDisplayValue(storedUrl), { target: { value: editedUrl } });
    expect(screen.getByTestId('profile-settings').textContent).toBe(`custom|${editedUrl}`);

    chooseSelectOption('sub-profile-mode', 'No link');
    expect(screen.getByTestId('profile-settings').textContent).toBe(`none|${editedUrl}`);
    expect(screen.queryByDisplayValue(editedUrl)).toBeNull();
    expect(screen.queryByText(warning)).toBeNull();

    chooseSelectOption('sub-profile-mode', 'Custom website');
    expect(screen.getByTestId('profile-settings').textContent).toBe(`custom|${editedUrl}`);
    expect(screen.getByDisplayValue(editedUrl)).toBeTruthy();
  });

  it('opens a legacy custom profile URL with custom mode selected', () => {
    const storedUrl = 'https://example.com/profile/{{SUB_ID}}';

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings#subscription']}>
        <ProfileSettingsHarness initial={{ subProfileUrl: storedUrl }} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('tab', { name: /Profile/ }));
    expect(screen.getByRole('combobox', { name: 'Profile page' })).toBeTruthy();
    expect(screen.getByText('Custom website')).toBeTruthy();
    expect(screen.getByDisplayValue(storedUrl)).toBeTruthy();
    expect(screen.getByTestId('profile-settings').textContent).toBe(`custom|${storedUrl}`);
  });

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
