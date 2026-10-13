import { describe, expect, it, vi, afterEach } from 'vitest';
import { act, fireEvent, screen, waitFor } from '@testing-library/react';

import WarpModal from '@/pages/xray/overrides/WarpModal';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

const registration = {
  privateKey: '-----BEGIN PRIVATE KEY-----\nAAA\n-----END PRIVATE KEY-----\n',
  publicKey: '-----BEGIN PUBLIC KEY-----\nBBB\n-----END PUBLIC KEY-----\n',
  address: ['172.16.0.2', '2606:4700:110:8a36::2'],
  endpoint: '162.159.198.1',
};

function mockApi() {
  vi.spyOn(HttpUtil, 'post').mockImplementation(async (url) => {
    if (url === '/panel/api/xray/warp/regMasque') {
      return new Msg(true, '', JSON.stringify(registration));
    }
    if (url === '/panel/api/setting/all') return new Msg(true, '', {});
    return new Msg(true, '', '');
  });
}

async function clickAddMasque() {
  const button = await screen.findByRole('button', { name: /Add WARP over MASQUE outbound/ });
  // The modal loads its WARP data on open; a loading button swallows the click.
  await waitFor(() => expect(button.className).not.toContain('ant-btn-loading'));
  await act(async () => {
    fireEvent.click(button);
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

// WARP over MASQUE dials Cloudflare's MASQUE endpoint with the enrolled key, its SNI
// and HTTP/3; an outbound missing any of them fails the handshake at runtime.
describe('WarpModal WARP over MASQUE', () => {
  const expected = {
    tag: 'warp-masque',
    protocol: 'masque',
    settings: { address: '162.159.198.1', port: 443 },
    streamSettings: {
      network: 'masque',
      security: 'tls',
      tlsSettings: { serverName: 'consumer-masque.cloudflareclient.com', alpn: ['h3'] },
      masqueSettings: {
        warp: {
          privateKey: registration.privateKey,
          publicKey: registration.publicKey,
          address: registration.address,
        },
      },
    },
  };

  it('adds a MASQUE outbound built from a fresh enrollment', async () => {
    mockApi();
    const onAddOutbound = vi.fn();
    renderWithProviders(
      <WarpModal
        open
        templateSettings={{ outbounds: [{ tag: 'direct' }] }}
        onClose={() => {}}
        onAddOutbound={onAddOutbound}
        onResetOutbound={() => {}}
        onRemoveOutbound={() => {}}
      />,
    );

    await clickAddMasque();

    await waitFor(() => expect(onAddOutbound).toHaveBeenCalledWith(expected));
  });

  it('replaces an existing warp-masque outbound instead of adding a second one', async () => {
    mockApi();
    const onAddOutbound = vi.fn();
    const onResetOutbound = vi.fn();
    renderWithProviders(
      <WarpModal
        open
        templateSettings={{ outbounds: [{ tag: 'direct' }, { tag: 'warp-masque' }] }}
        onClose={() => {}}
        onAddOutbound={onAddOutbound}
        onResetOutbound={onResetOutbound}
        onRemoveOutbound={() => {}}
      />,
    );

    await clickAddMasque();

    await waitFor(() =>
      expect(onResetOutbound).toHaveBeenCalledWith({ index: 1, outbound: expected }),
    );
    expect(onAddOutbound).not.toHaveBeenCalled();
  });
});
