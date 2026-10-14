import type { ReactNode } from 'react';
import { act, renderHook, screen, waitFor } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router';
import { afterEach, expect, test, vi } from 'vitest';

import { keys } from '@/api/queryKeys';
import { useAllSettings } from '@/api/queries/useAllSettings';
import { useSponsorsQuery } from '@/api/queries/useSponsorsQuery';
import SponsorSlot from '@/components/sponsor/SponsorSlot';
import CommandPalette from '@/components/command-palette/CommandPalette';
import { commandPaletteStore } from '@/components/command-palette/useCommandPalette';
import AppSidebar from '@/layouts/AppSidebar';
import SponsorsPage from '@/pages/sponsors/SponsorsPage';
import { HttpUtil, Msg } from '@/utils';
import { makeTestQueryClient, renderWithProviders } from './test-utils';

const ads = {
  contact: 'https://contact.example/',
  sponsors: [
    {
      id: 'acme',
      name: 'Acme',
      slots: ['login', 'dashboard', 'page'],
      until: '2099-01-01T00:00:00Z',
      link: 'https://acme.example/',
      logo: '/sponsors/logo/acme.png',
      title: { en: 'Acme sponsor' },
      text: { en: 'Ad text' },
    },
  ],
};

afterEach(() => {
  act(() => commandPaletteStore.close());
  vi.restoreAllMocks();
  localStorage.clear();
});

test.each([true, false])('sponsor search command follows enabled=%s', async (enabled) => {
  mockAPI(enabled);
  const client = makeTestQueryClient();
  renderWithProviders(
    <MemoryRouter>
      <CommandPalette />
    </MemoryRouter>,
    { queryClient: client },
  );
  await waitFor(() => expect(client.getQueryData(keys.sponsorVisibility())).toBe(enabled));
  act(() => commandPaletteStore.open());
  if (enabled) expect(screen.getByText('Sponsors')).toBeTruthy();
  else expect(screen.queryByText('Sponsors')).toBeNull();
});

function mockAPI(initial: boolean, saveSuccess = true) {
  let enabled = initial;
  const get = vi.spyOn(HttpUtil, 'get').mockImplementation(async (url) => {
    if (url === '/sponsors/enabled') return new Msg(true, '', enabled);
    if (url === '/sponsors') return new Msg(true, '', ads);
    return new Msg(true, '', []);
  });
  const post = vi.spyOn(HttpUtil, 'post').mockImplementation(async (url, body) => {
    if (url === '/panel/api/setting/update') {
      if (saveSuccess) enabled = (body as { showSponsorAds: boolean }).showSponsorAds;
      return new Msg(saveSuccess, '');
    }
    return new Msg(true, '', { showSponsorAds: enabled });
  });
  return { get, post };
}

test.each(['login', 'dashboard'] as const)(
  '%s hides warm cached ads and requests only the public flag when disabled',
  async (slot) => {
    const { get, post } = mockAPI(false);
    const client = makeTestQueryClient();
    client.setQueryData(keys.sponsors(), ads);
    const view = renderWithProviders(<SponsorSlot slot={slot} />, { queryClient: client });
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/sponsors/enabled', undefined, expect.any(Object)),
    );
    await waitFor(() => expect(client.getQueryData(keys.sponsorVisibility())).toBe(false));
    expect(view.container.textContent).toBe('');
    expect(view.container.querySelector('img')).toBeNull();
    expect(get.mock.calls.map(([url]) => url)).toEqual(['/sponsors/enabled']);
    expect(post).not.toHaveBeenCalled();
  },
);

test.each([true, false])(
  'successful save=%s updates visibility immediately and re-enable fetches sponsors again',
  async (success) => {
    mockAPI(true, success);
    const client = makeTestQueryClient();
    client.setQueryData(keys.sponsors(), ads);
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(
      () => ({ settings: useAllSettings(), sponsors: useSponsorsQuery() }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.settings.fetched).toBe(true));
    await waitFor(() => expect(result.current.sponsors.data.sponsors).toHaveLength(1));
    await act(async () => {
      await result.current.settings.savePayload({ showSponsorAds: false });
    });
    if (!success) {
      expect(result.current.sponsors.data.sponsors).toHaveLength(1);
      expect(client.getQueryData(keys.sponsorVisibility())).toBe(true);
      return;
    }
    expect(result.current.sponsors.data).toEqual({ sponsors: [] });
    expect(client.getQueryData(keys.sponsors())).toBeUndefined();
    expect(client.getQueryData(keys.sponsorVisibility())).toBe(false);
    await act(async () => {
      await result.current.settings.savePayload({ showSponsorAds: true });
    });
    await waitFor(() => expect(result.current.sponsors.data).toEqual(ads));
  },
);

test('a disabled direct sponsor URL redirects without cards or placement offers', async () => {
  const { get } = mockAPI(false);
  renderWithProviders(
    <MemoryRouter initialEntries={['/sponsors']}>
      <Routes>
        <Route path="/sponsors" element={<SponsorsPage />} />
        <Route path="/" element={<div>Dashboard destination</div>} />
      </Routes>
    </MemoryRouter>,
  );
  await screen.findByText('Dashboard destination');
  expect(screen.queryByText('Acme sponsor')).toBeNull();
  expect(document.querySelector('a[href="https://contact.example/"]')).toBeNull();
  expect(get.mock.calls.map(([url]) => url)).toEqual(['/sponsors/enabled']);
});

test('an in-flight sponsor response cannot restore ads after disabling', async () => {
  const { get } = mockAPI(true);
  let resolveAds: ((value: Msg<unknown>) => void) | undefined;
  const pending = new Promise<Msg<unknown>>((resolve) => {
    resolveAds = resolve;
  });
  get.mockImplementation(async (url) => (url === '/sponsors' ? pending : new Msg(true, '', true)));
  const client = makeTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  const { result } = renderHook(
    () => ({ settings: useAllSettings(), sponsors: useSponsorsQuery() }),
    { wrapper },
  );
  await waitFor(() => expect(result.current.settings.fetched).toBe(true));
  await waitFor(() => expect(get).toHaveBeenCalledWith('/sponsors', undefined, expect.any(Object)));
  await act(async () => {
    await result.current.settings.savePayload({ showSponsorAds: false });
  });
  await act(async () => {
    resolveAds?.(new Msg(true, '', ads));
    await pending;
  });
  expect(result.current.sponsors.data).toEqual({ sponsors: [] });
  expect(client.getQueryData(keys.sponsors())).toBeUndefined();
});

test.each([true, false])('sidebar sponsor navigation follows enabled=%s', async (enabled) => {
  const { get } = mockAPI(enabled);
  renderWithProviders(
    <MemoryRouter>
      <AppSidebar />
    </MemoryRouter>,
  );
  await waitFor(() =>
    expect(get).toHaveBeenCalledWith('/sponsors/enabled', undefined, expect.any(Object)),
  );
  if (enabled) await screen.findByText('Sponsors');
  else expect(screen.queryByText('Sponsors')).toBeNull();
});
