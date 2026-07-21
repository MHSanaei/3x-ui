import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { ThemeProvider } from '@/hooks/useTheme';
import ClientFormModal from '@/pages/clients/ClientFormModal';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

function makeQC() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

const REALITY_INBOUND = {
  id: 4,
  port: 10443,
  protocol: 'vless',
  tag: 'in-10443-tcp',
  tlsFlowCapable: true,
  enable: true,
} as unknown as InboundOption;

const CLIENT = {
  email: 'testuser',
  flow: 'xtls-rprx-vision',
  uuid: '11111111-1111-1111-1111-111111111111',
  subId: 'subid123',
  enable: true,
} as unknown as ClientRecord;

function savedFlow(save: ReturnType<typeof vi.fn>): unknown {
  return (save.mock.calls[0][0] as Record<string, unknown>).flow;
}

describe('ClientFormModal — Vision flow preservation', () => {
  it('keeps xtls-rprx-vision with a stable Reality inbound', async () => {
    const qc = makeQC();
    const save = vi.fn().mockResolvedValue({ success: true });
    render(
      <ThemeProvider>
        <QueryClientProvider client={qc}>
          <ClientFormModal open mode="edit" client={CLIENT} inbounds={[REALITY_INBOUND]} attachedIds={[4]} save={save} onOpenChange={() => {}} />
        </QueryClientProvider>
      </ThemeProvider>,
    );
    fireEvent.click(await screen.findByRole('button', { name: /save/i }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(savedFlow(save)).toBe('xtls-rprx-vision');
  });

  it('does not drop a selected Vision flow while the inbound options momentarily reload', async () => {
    const qc = makeQC();
    const save = vi.fn().mockResolvedValue({ success: true });
    const tree = (inbounds: InboundOption[]) => (
      <ThemeProvider>
        <QueryClientProvider client={qc}>
          <ClientFormModal open mode="edit" client={CLIENT} inbounds={inbounds} attachedIds={[4]} save={save} onOpenChange={() => {}} />
        </QueryClientProvider>
      </ThemeProvider>
    );
    // Options loaded -> reloading (inboundOptionsQuery.data ?? [] === []) -> loaded again.
    const { rerender } = render(tree([REALITY_INBOUND]));
    await screen.findByRole('button', { name: /save/i });
    rerender(tree([]));
    rerender(tree([REALITY_INBOUND]));

    fireEvent.click(await screen.findByRole('button', { name: /save/i }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(savedFlow(save)).toBe('xtls-rprx-vision');
  });
});
