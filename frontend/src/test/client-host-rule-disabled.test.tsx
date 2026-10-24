import { describe, it, expect, vi } from 'vitest';
import type { ComponentProps } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { ThemeProvider } from '@/hooks/useTheme';
import ClientFormModal from '@/pages/clients/ClientFormModal';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

vi.mock('@/api/queries/useRawHostsQuery', () => ({
  useRawHostsQuery: () => ({
    hosts: [
      { id: 99, inboundId: 1, remark: 'cdn-disabled', address: 'c.example.com', isDisabled: true },
      { id: 100, inboundId: 1, remark: 'cdn-live', address: 'live.example.com', isDisabled: false },
      { id: 200, inboundId: 2, remark: 'other', address: 'o.example.com', isDisabled: false },
    ],
    loading: false,
    fetched: true,
    fetchError: '',
    refetch: vi.fn(),
  }),
}));

function makeQC() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

const INBOUNDS = [
  { id: 1, port: 443, protocol: 'vless', tag: 'in-1', remark: 'one', enable: true },
  { id: 2, port: 8443, protocol: 'vless', tag: 'in-2', remark: 'two', enable: true },
] as unknown as InboundOption[];

const CLIENT = {
  email: 'bound@example.com',
  uuid: '11111111-1111-1111-1111-111111111111',
  subId: 'subid123',
  enable: true,
  clientHostRuleIds: [99],
} as unknown as ClientRecord;

function renderEdit(save: ComponentProps<typeof ClientFormModal>['save']) {
  const qc = makeQC();
  return {
    save,
    view: (
      <ThemeProvider>
        <QueryClientProvider client={qc}>
          <ClientFormModal
            open
            mode="edit"
            client={CLIENT}
            inbounds={INBOUNDS}
            attachedIds={[1]}
            save={save}
            onOpenChange={() => {}}
          />
        </QueryClientProvider>
      </ThemeProvider>
    ),
  };
}

describe('ClientFormModal — disabled host binding preservation', () => {
  it('keeps a disabled host binding when saving an unrelated field', async () => {
    const save = vi.fn().mockResolvedValue({ success: true });
    const { view } = renderEdit(save);
    render(view);

    const comment = await screen.findByLabelText('Comment');
    fireEvent.change(comment, { target: { value: 'touched' } });

    fireEvent.click(await screen.findByRole('button', { name: /save/i }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    const payload = save.mock.calls[0][0] as Record<string, unknown>;
    expect(payload.clientHostRuleIds).toContain(99);
  });
});
