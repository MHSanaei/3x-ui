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
      { id: 99, inboundId: 1, remark: 'cdn-a', address: 'a.example.com', isDisabled: false },
      { id: 100, inboundId: 1, remark: 'cdn-b', address: 'b.example.com', isDisabled: false },
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
] as unknown as InboundOption[];

// List-only payload; single mirrors the list for compatibility.
const CLIENT = {
  email: 'list-only@example.com',
  uuid: '11111111-1111-1111-1111-111111111111',
  subId: 'subid123',
  enable: true,
  clientHostRuleId: null,
  clientHostRuleIds: [100],
} as unknown as ClientRecord;

function renderEdit(save: ComponentProps<typeof ClientFormModal>['save']) {
  const qc = makeQC();
  return (
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
  );
}

describe('ClientFormModal — list-only host payload', () => {
  it('sends the list as the value with the single mirroring its first id', async () => {
    const save = vi.fn().mockResolvedValue({ success: true });
    render(renderEdit(save));

    const comment = await screen.findByLabelText('Comment');
    fireEvent.change(comment, { target: { value: 'touched' } });

    fireEvent.click(await screen.findByRole('button', { name: /save/i }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    const payload = save.mock.calls[0][0] as Record<string, unknown>;
    expect(payload.clientHostRuleIds).toEqual([100]);
    expect(payload.clientHostRuleId).toBe(100);
  });
});
