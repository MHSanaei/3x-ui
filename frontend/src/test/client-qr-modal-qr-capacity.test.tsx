import { fireEvent, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router';

import type { HappLinkResult } from '@/generated/types';
import type { ClientRecord } from '@/hooks/useClients';
import ClientQrModal from '@/pages/clients/ClientQrModal';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

const CLIENT: ClientRecord = { id: 42, email: 'alice@example.com', subId: 'alpha' };
const SUB_SETTINGS = {
  enable: true,
  subURI: 'https://panel.example/sub/',
  subJsonURI: '',
  subJsonEnable: false,
  happLinkEnable: true,
};
// QrPanel encodes at error level L; QR version 40 holds 2953 bytes at that level.
const LEVEL_L_CAPACITY_BYTES = 2953;

function happLinkOfBytes(bytes: number) {
  const prefix = 'happ://crypt5/';
  return prefix + 'a'.repeat(bytes - prefix.length);
}

function renderHappVariant(link: string) {
  vi.mocked(HttpUtil.post).mockResolvedValue(
    new Msg<HappLinkResult>(true, '', { encryptedLink: link }),
  );
  renderWithProviders(
    <MemoryRouter initialEntries={['/clients']}>
      <ClientQrModal
        open
        client={CLIENT}
        inboundsById={{}}
        subSettings={SUB_SETTINGS}
        onOpenChange={() => {}}
      />
    </MemoryRouter>,
  );
  fireEvent.click(screen.getByRole('radio', { name: /Happ Encrypted Link/ }));
}

describe('ClientQrModal Happ QR capacity against the real encoder', () => {
  beforeEach(() => {
    vi.mocked(HttpUtil.post).mockReset();
  });

  it('renders the QR for a link exactly at the level-L capacity', async () => {
    renderHappVariant(happLinkOfBytes(LEVEL_L_CAPACITY_BYTES));

    await screen.findByRole('button', { name: 'Regenerate' });
    expect(document.body.querySelector('.qr-panel-canvas svg')).not.toBeNull();
    expect(screen.queryByText(/too long to display as a QR code/)).toBeNull();
  });

  it('keeps a link one byte over the capacity available without a QR', async () => {
    renderHappVariant(happLinkOfBytes(LEVEL_L_CAPACITY_BYTES + 1));

    await screen.findByRole('button', { name: 'Regenerate' });
    expect(document.body.querySelector('.qr-panel-canvas')).toBeNull();
    expect(screen.getByText(/too long to display as a QR code/)).toBeTruthy();
  });
});
