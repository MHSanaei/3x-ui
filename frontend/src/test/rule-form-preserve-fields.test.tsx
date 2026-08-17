import { describe, it, expect, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import RuleFormModal from '@/pages/xray/routing/RuleFormModal';

import { renderWithProviders } from './test-utils';

describe('RuleFormModal edit preserves unsurfaced fields', () => {
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
});
