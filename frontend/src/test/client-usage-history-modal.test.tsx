import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';

import ClientUsageHistoryModal from '@/components/clients/ClientUsageHistoryModal';
import { HttpUtil } from '@/utils';

const GiB = 1024 ** 3;

const history = {
  email: 'client@example.com',
  lifetimeUp: 10 * GiB,
  lifetimeDown: 20 * GiB,
  currentUp: GiB,
  currentDown: 2 * GiB,
  windows: [
    {
      id: 1,
      email: 'client@example.com',
      at: 1735689600000,
      up: 9 * GiB,
      down: 18 * GiB,
      reason: 'renew',
    },
  ],
  nodes: [{ nodeId: 7, nodeName: 'de-fra-1', up: 3 * GiB, down: 4 * GiB }],
  changes: [
    {
      id: 1,
      email: 'client@example.com',
      at: 1735776000000,
      field: 'dataLimit',
      oldValue: String(10 * GiB),
      newValue: String(20 * GiB),
      source: 'panel',
    },
  ],
};

function openModal() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ClientUsageHistoryModal open email="client@example.com" onClose={() => {}} />
    </QueryClientProvider>,
  );
}

describe('ClientUsageHistoryModal', () => {
  it('shows lifetime totals, per-node usage and the change log', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue({
      success: true,
      msg: '',
      obj: history,
    });

    openModal();

    // The content sections appear once the (mocked) fetch resolves.
    await waitFor(() => {
      expect(screen.getByText('Lifetime traffic')).toBeTruthy();
    });
    // The modal title carries the email; the rest of the sections:
    expect(screen.getByText(/Usage & change history/)).toBeTruthy();
    expect(screen.getByText('Lifetime traffic')).toBeTruthy();
    expect(screen.getByText(/^Current cycle/)).toBeTruthy();
    // The per-node section carries its own up/down columns.
    expect(screen.getByText('Usage by node')).toBeTruthy();
    expect(screen.getByText('de-fra-1')).toBeTruthy();
    // The change log renders the renewed data limit with its source.
    expect(screen.getByText('Changes')).toBeTruthy();
    expect(screen.getByText('panel')).toBeTruthy();
  });

  it('renders the closed cycles table with the renew reason', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue({
      success: true,
      msg: '',
      obj: history,
    });

    openModal();

    await waitFor(() => {
      expect(screen.getByText('Past cycles')).toBeTruthy();
    });
    expect(screen.getByText('renewed')).toBeTruthy();
  });

  it('shows the empty state for a client without history', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue({
      success: true,
      msg: '',
      obj: {
        email: 'fresh@example.com',
        lifetimeUp: 0,
        lifetimeDown: 0,
        currentUp: 0,
        currentDown: 0,
        windows: [],
        nodes: [],
        changes: [],
      },
    });

    openModal();

    await waitFor(() => {
      expect(screen.getByText('No usage or change history yet.')).toBeTruthy();
    });
    expect(screen.queryByText('Usage by node')).toBeNull();
  });
});
