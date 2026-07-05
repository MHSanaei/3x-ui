import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { QueryProvider } from '@/api/QueryProvider';
import ClientFormModal from '@/pages/clients/ClientFormModal';
import { renderWithProviders } from './test-utils';

function renderModal() {
  return renderWithProviders(
    <QueryProvider>
      <ClientFormModal
        open
        mode="add"
        client={null}
        inbounds={[]}
        save={async () => null}
        onOpenChange={() => {}}
      />
    </QueryProvider>,
  );
}

describe('ClientFormModal', () => {
  it('explains which protocol credentials use password and auth fields', () => {
    renderModal();

    fireEvent.click(screen.getByText('Credentials'));

    expect(screen.getByText('Used for Trojan and Shadowsocks clients. Ignored by other protocols.')).toBeTruthy();
    expect(screen.getByText('Credential used by Hysteria clients. This is separate from Password.')).toBeTruthy();
  });
});
