import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { ThemeProvider } from '@/hooks/useTheme';
import ClientFormModal from '@/pages/clients/ClientFormModal';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

const VLESS_INBOUND = {
  id: 7,
  port: 10443,
  protocol: 'vless',
  tag: 'in-10443-tcp',
  enable: true,
} as unknown as InboundOption;

const CLIENT = {
  email: 'reverse-user',
  uuid: '11111111-1111-1111-1111-111111111111',
  subId: 'subid-reverse',
  enable: true,
  reverse: { tag: 'back' },
} as unknown as ClientRecord;

async function saveEdited(edit?: (input: HTMLInputElement) => void) {
  const save = vi.fn().mockResolvedValue({ success: true });
  render(
    <ThemeProvider>
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <ClientFormModal
          open
          mode="edit"
          client={CLIENT}
          inbounds={[VLESS_INBOUND]}
          attachedIds={[7]}
          save={save}
          onOpenChange={() => {}}
        />
      </QueryClientProvider>
    </ThemeProvider>,
  );
  const input = (await screen.findByPlaceholderText('Optional reverse tag')) as HTMLInputElement;
  await waitFor(() => expect(input.value).toBe('back'));
  edit?.(input);
  fireEvent.click(await screen.findByRole('button', { name: /save/i }));
  await waitFor(() => expect(save).toHaveBeenCalled());
  return save.mock.calls[0][0] as Record<string, unknown>;
}

// A cleared tag goes out as an explicit null rather than a missing key, which a
// partial update would read as "keep the stored tag".
describe('ClientFormModal — reverse tag', () => {
  it('sends an explicit null when the operator clears the tag', async () => {
    const payload = await saveEdited((input) => fireEvent.change(input, { target: { value: '' } }));
    expect(payload).toHaveProperty('reverse', null);
  });

  it('round-trips an unchanged tag', async () => {
    const payload = await saveEdited();
    expect(payload.reverse).toEqual({ tag: 'back' });
  });
});
