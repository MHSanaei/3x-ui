import { useState } from 'react';
import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import GeneralTab from '@/pages/settings/GeneralTab';
import { HttpUtil, Msg } from '@/utils';
import { chooseSelectOption, renderWithProviders } from './test-utils';

function SettingsHarness({ initial }: { initial?: unknown }) {
  const [settings, setSettings] = useState(() => new AllSetting(initial));
  return (
    <MemoryRouter>
      <GeneralTab
        allSetting={settings}
        updateSetting={(patch) =>
          setSettings((current) => new AllSetting({ ...current, ...patch }))
        }
      />
      <output data-testid="settings">{JSON.stringify(settings)}</output>
    </MemoryRouter>
  );
}

afterEach(() => vi.restoreAllMocks());

function mockInterfaces(names: string[], automatic = 'eth0', automaticError = '') {
  vi.spyOn(HttpUtil, 'get').mockImplementation(
    async (url) =>
      new Msg(
        true,
        '',
        url === '/panel/api/server/networkInterfaces'
          ? {
              interfaces: names.map((name) => ({ name, up: true })),
              automatic,
              automaticError,
            }
          : [],
      ),
  );
}

describe('network traffic settings', () => {
  it('shows the automatic interface and reports detection failure without substituting legacy mode', async () => {
    mockInterfaces(['eth0'], 'eth0');
    const { unmount } = renderWithProviders(
      <SettingsHarness initial={{ networkTrafficMode: 'auto' }} />,
    );
    await screen.findByText('Default-route interface: eth0');
    unmount();
    mockInterfaces(['eth0'], '', 'no default route');
    renderWithProviders(<SettingsHarness initial={{ networkTrafficMode: 'auto' }} />);
    await screen.findByText(
      'No default-route interface is available. Traffic metrics remain unavailable until detection recovers or the mode changes.',
    );
    expect(JSON.parse(screen.getByTestId('settings').textContent || '{}').networkTrafficMode).toBe(
      'auto',
    );
  });

  it('shows loading and an empty result without fabricating interface options', async () => {
    let complete: (value: Msg<unknown>) => void = () => undefined;
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url) =>
      url === '/panel/api/server/networkInterfaces'
        ? new Promise<Msg<unknown>>((resolve) => {
            complete = resolve;
          })
        : new Msg(true, '', []),
    );
    renderWithProviders(<SettingsHarness initial={{ networkTrafficMode: 'manual' }} />);
    expect(
      (document.getElementById('network-traffic-interface') as HTMLInputElement).disabled,
    ).toBe(true);
    await act(async () =>
      complete(
        new Msg(true, '', { interfaces: [], automatic: '', automaticError: 'no default route' }),
      ),
    );
    await screen.findByText('No network interfaces are available.');
  });

  it('can retry an interface-list error and recover the automatic interface', async () => {
    let attempts = 0;
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url) => {
      if (url !== '/panel/api/server/networkInterfaces') return new Msg(true, '', []);
      if (attempts++ === 0) throw new Error('offline');
      return new Msg(true, '', {
        interfaces: [{ name: 'eth0', up: true }],
        automatic: 'eth0',
        automaticError: '',
      });
    });
    renderWithProviders(<SettingsHarness initial={{ networkTrafficMode: 'auto' }} />);
    await screen.findByText('Could not load network interfaces.');
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    await screen.findByText('Default-route interface: eth0');
    expect(screen.queryByText('Could not load network interfaces.')).toBeNull();
  });
  it('keeps legacy as the default and selects exactly the requested dynamic interface', async () => {
    mockInterfaces(['eth0', 'ens3', 'enP6129s1', 'wg0']);
    renderWithProviders(<SettingsHarness />);
    expect(screen.getByText('All interfaces (Legacy)')).toBeTruthy();
    chooseSelectOption('network-traffic-mode', 'Manual');
    await waitFor(() =>
      expect(
        (document.getElementById('network-traffic-interface') as HTMLInputElement).disabled,
      ).toBe(false),
    );
    chooseSelectOption('network-traffic-interface', 'enP6129s1');
    expect(JSON.parse(screen.getByTestId('settings').textContent || '{}')).toMatchObject({
      networkTrafficMode: 'manual',
      networkTrafficInterface: 'enP6129s1',
    });
    chooseSelectOption('network-traffic-interface', 'wg0');
    expect(
      JSON.parse(screen.getByTestId('settings').textContent || '{}').networkTrafficInterface,
    ).toBe('wg0');
  });

  it('preserves a saved missing interface and warns instead of replacing it', async () => {
    mockInterfaces(['eth0']);
    renderWithProviders(
      <SettingsHarness
        initial={{ networkTrafficMode: 'manual', networkTrafficInterface: 'removed0' }}
      />,
    );
    await screen.findByText(
      'The selected network interface is unavailable. Choose an available interface or another monitoring mode.',
    );
    expect(
      JSON.parse(screen.getByTestId('settings').textContent || '{}').networkTrafficInterface,
    ).toBe('removed0');
    chooseSelectOption('network-traffic-interface', 'eth0');
    await waitFor(() =>
      expect(
        screen.queryByText(
          'The selected network interface is unavailable. Choose an available interface or another monitoring mode.',
        ),
      ).toBeNull(),
    );
  });
});
