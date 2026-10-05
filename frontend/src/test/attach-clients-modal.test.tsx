import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';

import AttachClientsModal from '@/pages/inbounds/clients/AttachClientsModal';
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

describe('AttachClientsModal', () => {
  it('lists the source clients, selected, when first mounted already open', async () => {
    renderWithProviders(
      <AttachClientsModal open source={sourceInbound()} dbInbounds={[]} onClose={() => {}} />,
    );

    expect(await screen.findByText('alice@test')).toBeTruthy();
    expect(screen.getByText('bob@test')).toBeTruthy();

    const boxes = screen.getAllByRole('checkbox') as HTMLInputElement[];
    const rowBoxes = boxes.slice(1);
    expect(rowBoxes).toHaveLength(2);
    expect(rowBoxes.every((b) => b.checked)).toBe(true);
  });
});
