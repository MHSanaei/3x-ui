import { describe, it, expect, vi } from 'vitest';
import { fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import ClientFormModal from '@/pages/clients/ClientFormModal';
import { renderWithProviders } from './test-utils';

// ClientFormModal reads server state via react-query (useFail2banStatusQuery),
// so it needs a QueryClientProvider on top of the shared ThemeProvider wrapper.
function renderModal() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithProviders(
    <QueryClientProvider client={queryClient}>
      <ClientFormModal
        open
        mode="add"
        client={null}
        inbounds={[]}
        save={vi.fn().mockResolvedValue(null)}
        onOpenChange={() => {}}
      />
    </QueryClientProvider>,
  );
}

function openCredentialsTab() {
  const tab = Array.from(document.querySelectorAll('.ant-tabs-tab'))
    .find((t) => (t.textContent ?? '').trim() === 'Credentials');
  if (!tab) throw new Error('Credentials tab not found');
  fireEvent.click(tab);
}

function tooltipIconForLabel(label: string): HTMLElement {
  const labelEl = Array.from(document.querySelectorAll('.ant-form-item-label label'))
    .find((l) => (l.textContent ?? '').trim() === label);
  const item = labelEl?.closest('.ant-form-item') as HTMLElement | null;
  if (!item) throw new Error(`Form item not found for label: ${label}`);
  const tip = item.querySelector('.ant-form-item-tooltip') as HTMLElement | null;
  if (!tip) throw new Error(`No tooltip on form item: ${label}`);
  return tip;
}

describe('ClientFormModal credential tooltips', () => {
  it('explains that the Password field is only consumed by Trojan/Shadowsocks', async () => {
    renderModal();
    openCredentialsTab();

    const tip = tooltipIconForLabel('Password');
    fireEvent.mouseEnter(tip);

    await waitFor(() => {
      expect(document.body.textContent).toContain(
        'Only used by Trojan and Shadowsocks clients; ignored for VLESS, VMess, Hysteria, and WireGuard.',
      );
    });
  });

  it('explains that Hysteria Auth is the credential Hysteria actually uses', async () => {
    renderModal();
    openCredentialsTab();

    const tip = tooltipIconForLabel('Hysteria Auth');
    fireEvent.mouseEnter(tip);

    await waitFor(() => {
      expect(document.body.textContent).toContain(
        'Credential used only by Hysteria clients. Trojan and Shadowsocks use the Password field instead.',
      );
    });
  });
});
