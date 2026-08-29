import { screen } from '@testing-library/react';
import dayjs from 'dayjs';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import FrontProxyTab from '@/pages/settings/FrontProxyTab';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

// FrontProxyTab also renders DefaultSettingTag, which fires its own
// HttpUtil.post('/panel/api/setting/factoryDefaults', ...) query on mount.
// mockResolvedValueOnce would race the two calls and could hand the queued
// status payload to the wrong one, so this discriminates by URL instead.
function mockStatus(cert?: Record<string, unknown>) {
  vi.spyOn(HttpUtil, 'post').mockImplementation(async (url: string) => {
    if (!url.includes('/frontproxy/status')) return new Msg(true, '', {});
    return new Msg(true, '', {
      running: true,
      port: 7443,
      templates: [],
      decoyUploaded: false,
      cert,
    });
  });
}

describe('FrontProxyTab certificate status', () => {
  it('interpolates the expiry date instead of leaving the raw placeholder', async () => {
    mockStatus({ state: 'obtained', domain: 'example.com', notAfter: '2027-01-15T10:30:00' });

    renderWithProviders(<FrontProxyTab allSetting={new AllSetting({})} updateSetting={vi.fn()} />);

    const expected = dayjs('2027-01-15T10:30:00').format('YYYY-MM-DD HH:mm:ss');
    expect(await screen.findByText(`Valid until ${expected}`)).toBeTruthy();
    expect(screen.queryByText(/\{date\}/)).toBeNull();
  });

  it('renders a spinner while the certificate is being obtained', async () => {
    mockStatus({ state: 'obtaining', domain: 'example.com' });

    renderWithProviders(<FrontProxyTab allSetting={new AllSetting({})} updateSetting={vi.fn()} />);

    expect(await screen.findByText('Obtaining the certificate…')).toBeTruthy();
  });

  it('renders the error text when the certificate could not be obtained', async () => {
    mockStatus({ state: 'failed', domain: 'example.com', error: 'no route to host' });

    renderWithProviders(<FrontProxyTab allSetting={new AllSetting({})} updateSetting={vi.fn()} />);

    expect(await screen.findByText('Could not obtain the certificate')).toBeTruthy();
    expect(screen.getByText('no route to host')).toBeTruthy();
  });

  it('renders nothing when the backend response has no cert field yet', async () => {
    mockStatus(undefined);

    renderWithProviders(<FrontProxyTab allSetting={new AllSetting({})} updateSetting={vi.fn()} />);

    expect(await screen.findByText('Running')).toBeTruthy();
    expect(screen.queryByText('Certificate status')).toBeNull();
  });
});
