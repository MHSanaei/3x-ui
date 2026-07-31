import type { ReactNode } from 'react';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import GeoBrowserModal from '@/components/geodata/GeoBrowserModal';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

afterEach(() => {
  vi.restoreAllMocks();
});

const FILES = [{ name: 'geosite.dat', kind: 'site', size: 1024, modifiedAt: 1785428467270, categories: 3 }];

const IP_FILE = { name: 'geoip.dat', kind: 'ip', size: 2048, modifiedAt: 1785428467270, categories: 1 };

const IP_CATEGORIES = { total: 1, items: [{ code: 'private', entries: 1, attributes: [] }] };

const CATEGORIES = {
  total: 3,
  items: [
    { code: 'cn', entries: 2, attributes: [] },
    { code: 'google', entries: 2, attributes: ['ads'] },
    { code: 'telegram', entries: 1, attributes: [] },
  ],
};

function mockGeodata(files: unknown[] = FILES) {
  vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string, params?: unknown) => {
    const requestedFile = (params as { file?: string } | undefined)?.file;
    if (url.includes('/geodata/files')) return new Msg(true, '', files);
    if (url.includes('/geodata/categories')) {
      return new Msg(true, '', requestedFile === 'geoip.dat' ? IP_CATEGORIES : CATEGORIES);
    }
    if (url.includes('/geodata/entries')) return new Msg(true, '', { total: 0, items: [] });
    return new Msg(true, '', null);
  });
  vi.spyOn(HttpUtil, 'post').mockImplementation(async () => new Msg(true, '', []));
}

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={makeTestQueryClient()}>{children}</QueryClientProvider>;
}

async function checkboxFor(code: string) {
  const cell = await screen.findByText(code);
  const row = cell.closest('.ant-table-row');
  if (!row) throw new Error(`row for ${code} not found`);
  return within(row as HTMLElement).getByRole('checkbox') as HTMLInputElement;
}

describe('GeoBrowserModal selection', () => {
  it('seeds the selection from the field every time it opens', async () => {
    mockGeodata();
    const view = render(
      <GeoBrowserModal open kind="site" value="geosite:google" onApply={vi.fn()} onClose={vi.fn()} />,
      { wrapper },
    );

    await waitFor(async () => expect((await checkboxFor('google')).checked).toBe(true));

    view.rerender(
      <GeoBrowserModal open={false} kind="site" value="geosite:google" onApply={vi.fn()} onClose={vi.fn()} />,
    );
    view.rerender(
      <GeoBrowserModal open kind="site" value="geosite:google" onApply={vi.fn()} onClose={vi.fn()} />,
    );

    await waitFor(async () => expect((await checkboxFor('google')).checked).toBe(true));
    expect((await checkboxFor('cn')).checked).toBe(false);
  });

  it('keeps selections that the search box has filtered out of view', async () => {
    mockGeodata();
    const user = userEvent.setup();
    const onApply = vi.fn();
    render(<GeoBrowserModal open kind="site" value="" onApply={onApply} onClose={vi.fn()} />, { wrapper });

    await user.click(await checkboxFor('google'));
    await user.type(screen.getByPlaceholderText(/search category|поиск категории/i), 'cn');
    await waitFor(() => expect(screen.queryByText('google')).toBeNull());
    await user.click(await checkboxFor('cn'));

    await user.click(screen.getByRole('button', { name: /apply|применить/i }));

    expect(onApply).toHaveBeenCalledTimes(1);
    const applied = String(onApply.mock.calls[0][0]);
    expect(applied.split(',').map((token) => token.trim()).sort()).toEqual(['geosite:cn', 'geosite:google']);
  });

  it('drops a category from the field when its checkbox is cleared', async () => {
    mockGeodata();
    const user = userEvent.setup();
    const onApply = vi.fn();
    render(
      <GeoBrowserModal open kind="site" value="google.com, geosite:google, geosite:blabla" onApply={onApply} onClose={vi.fn()} />,
      { wrapper },
    );

    await waitFor(async () => expect((await checkboxFor('google')).checked).toBe(true));
    await user.click(await checkboxFor('google'));
    await user.click(screen.getByRole('button', { name: /apply|применить/i }));

    expect(onApply).toHaveBeenCalledWith('google.com, geosite:blabla');
  });

  it('offers only databases matching the field kind', async () => {
    mockGeodata([...FILES, IP_FILE]);
    render(<GeoBrowserModal open kind="ip" value="" onApply={vi.fn()} onClose={vi.fn()} />, { wrapper });

    await screen.findByText('private');
    expect(screen.getByTitle('geoip.dat')).toBeTruthy();
    expect(screen.queryByText('google')).toBeNull();
  });

  it('does not seed one database from another database categories', async () => {
    mockGeodata([...FILES, IP_FILE]);
    const user = userEvent.setup();
    render(
      <GeoBrowserModal open kind="site" value="geosite:cn" onApply={vi.fn()} onClose={vi.fn()} />,
      { wrapper },
    );

    await waitFor(async () => expect((await checkboxFor('cn')).checked).toBe(true));
    await user.click(await checkboxFor('google'));
    await waitFor(async () => expect((await checkboxFor('google')).checked).toBe(true));
    expect(screen.queryByText('private')).toBeNull();
  });

  it('ticks and unticks a category written in its long ext form', async () => {
    mockGeodata();
    const user = userEvent.setup();
    const onApply = vi.fn();
    render(
      <GeoBrowserModal
        open
        kind="site"
        value="ext:geosite.dat:cn, google.com"
        onApply={onApply}
        onClose={vi.fn()}
      />,
      { wrapper },
    );

    await waitFor(async () => expect((await checkboxFor('cn')).checked).toBe(true));
    await user.click(await checkboxFor('cn'));
    await user.click(screen.getByRole('button', { name: /apply|применить/i }));

    expect(onApply).toHaveBeenCalledWith('google.com');
  });
});
