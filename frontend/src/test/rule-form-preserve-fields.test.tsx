import { afterEach, describe, it, expect, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import RuleFormModal from '@/pages/xray/routing/RuleFormModal';
import { keys } from '@/api/queryKeys';
import { HttpUtil, Msg } from '@/utils';

import { chooseSelectOption, renderWithProviders } from './test-utils';

describe('RuleFormModal edit preserves unsurfaced fields', () => {
  afterEach(() => vi.restoreAllMocks());

  it('keeps a field the form does not surface (ruleTag) when saving an edit', () => {
    const onConfirm = vi.fn();
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    renderWithProviders(
      <QueryClientProvider client={queryClient}>
        <RuleFormModal
          open
          rule={{ type: 'field', outboundTag: 'block', ruleTag: 'my-tag', enabled: true }}
          inboundTags={[]}
          outboundTags={['block']}
          balancerTags={[]}
          onClose={vi.fn()}
          onConfirm={onConfirm}
        />
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm.mock.calls[0][0]).toMatchObject({ ruleTag: 'my-tag' });
  });

  it('selects existing clients for the user criterion', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      new Msg(true, '', [
        { id: 1, email: 'alice@example.com' },
        { id: 2, email: 'bob@example.com' },
      ]),
    );
    const onConfirm = vi.fn();
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    renderWithProviders(
      <RuleFormModal
        open
        rule={null}
        inboundTags={[]}
        outboundTags={['direct']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
      { queryClient },
    );

    await waitFor(() =>
      expect(HttpUtil.get).toHaveBeenCalledWith('/panel/api/clients/list', undefined, {
        silent: true,
      }),
    );
    await waitFor(() => expect(queryClient.getQueryData(keys.clients.all())).toHaveLength(2));
    const userField = screen.getByLabelText('User');
    chooseSelectOption(userField.id, 'alice@example.com');
    chooseSelectOption(userField.id, 'bob@example.com');
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    expect(onConfirm).toHaveBeenCalledWith(
      expect.objectContaining({ user: ['alice@example.com', 'bob@example.com'] }),
    );
  });
});
