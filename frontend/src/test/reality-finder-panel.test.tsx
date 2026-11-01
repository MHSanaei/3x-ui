import { describe, expect, it, vi } from 'vitest';
import { fireEvent, waitFor } from '@testing-library/react';

import RealityFinderPanel from '@/pages/inbounds/form/security/RealityFinderPanel';
import type { RealityFinderReport } from '@/generated/types';
import { HttpUtil, Msg } from '@/utils';
import { makeTestQueryClient, renderWithProviders } from './test-utils';

const command = new Msg(true, '', {
  token: 'tok',
  expiresAt: 1793000000,
  server: '203.0.113.10',
  scriptUrl: 'https://panel.example.com/realityfinder/run/tok/script.py',
  unix: 'python3 -c "run-unix"',
  windows: 'python -c "run-windows"',
});

function report(over: Partial<RealityFinderReport> = {}): RealityFinderReport {
  return {
    at: 1793000100,
    network: { ip: '5.120.1.2', asn: 197207, org: 'MCI', country: 'IR' },
    warning: '',
    router: false,
    tested: 253,
    server: { ip: '203.0.113.10', asn: 24940, holder: 'HETZNER-AS', prefix: '203.0.113.0/24' },
    port: 443,
    filterTested: true,
    targets: [
      {
        sni: 'open.example.com',
        ip: '203.0.113.20',
        score: 96,
        ms: 240,
        filtered: false,
        same24: true,
        sameAsn: true,
        cdn: '',
        dnsMatch: true,
        issuer: 'Sectigo',
        expires: '',
      },
      {
        sni: 'cut.example.com',
        ip: '203.0.113.21',
        score: 20,
        ms: 200,
        filtered: true,
        same24: true,
        sameAsn: true,
        cdn: '',
        dnsMatch: false,
        issuer: 'Sectigo',
        expires: '',
      },
    ],
    ...over,
  };
}

function renderPanel(listen: string, onPick = vi.fn()) {
  const view = renderWithProviders(
    <RealityFinderPanel
      inboundPort={8443}
      listen={listen}
      currentTarget="www.old.example.com:443"
      onPick={onPick}
    />,
    { queryClient: makeTestQueryClient() },
  );
  return { ...view, onPick };
}

describe('REALITY finder (scan from the admin device)', () => {
  it('asks for a command through 443 when the inbound sits behind a local listener', async () => {
    const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue(command);
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', []));
    const { getByRole, findByText } = renderPanel('127.0.0.1');

    fireEvent.click(getByRole('button', { name: 'Make command' }));

    expect(await findByText('python -c "run-windows"')).toBeTruthy();
    expect(post).toHaveBeenCalledWith(
      '/panel/api/server/realityFinder/command',
      { server: '', port: 443, scope: '24', current: 'www.old.example.com' },
      expect.anything(),
    );
  });

  it('shows the report and puts the picked name into the inbound', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(command);
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', [report()]));
    const { getByRole, findByText, getAllByRole, onPick } = renderPanel('');

    fireEvent.click(getByRole('button', { name: 'Make command' }));

    expect(await findByText('cut.example.com')).toBeTruthy();
    expect(await findByText('Filtered')).toBeTruthy();
    fireEvent.click(getAllByRole('button', { name: 'Use' })[0]);
    expect(onPick).toHaveBeenCalledWith('open.example.com:443', ['open.example.com']);
  });

  it('warns that a report from a VPN does not hold for the users', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(command);
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', [report({ warning: 'vpn' })]));
    const { getByRole, findByText } = renderPanel('');

    fireEvent.click(getByRole('button', { name: 'Make command' }));

    await waitFor(async () => expect(await findByText(/came from a VPN or a server/)).toBeTruthy());
  });
});
