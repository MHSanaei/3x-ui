import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';

import DetachClientsModal from '@/pages/inbounds/clients/DetachClientsModal';
import { DBInbound } from '@/models/dbinbound';

import { renderWithProviders } from './test-utils';

function sourceInbound() {
  return new DBInbound({
    id: 7,
    port: 443,
    listen: '',
    protocol: 'vless',
    remark: 'edge',
    enable: true,
    settings: JSON.stringify({
      clients: [
        { id: 'uuid-1', email: 'alice@test' },
        { id: 'uuid-2', email: 'bob@test' },
      ],
      decryption: 'none',
    }),
    streamSettings: JSON.stringify({ network: 'tcp', security: 'none' }),
    sniffing: '',
  });
}

describe('DetachClientsModal', () => {
  it('lists the attached clients when first mounted already open', async () => {
    renderWithProviders(<DetachClientsModal open source={sourceInbound()} onClose={() => {}} />);

    expect(await screen.findByText('alice@test')).toBeTruthy();
    expect(screen.getByText('bob@test')).toBeTruthy();
  });
});
