import { describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';

import PsiphonModal from '@/pages/xray/overrides/PsiphonModal';
import { renderWithProviders } from './test-utils';

describe('PsiphonModal', () => {
  it('renders modal with title and controls when open', async () => {
    renderWithProviders(<PsiphonModal open onClose={vi.fn()} />);

    expect(await screen.findByText(/Psiphon Outbounds/i)).toBeTruthy();
    expect(screen.getByText(/Test All Latencies/i)).toBeTruthy();
    expect(screen.getByText(/Psiphon Config/i)).toBeTruthy();
    expect(screen.getByText(/Select All Supported/i)).toBeTruthy();
  });

  it('does not render content when closed', () => {
    renderWithProviders(<PsiphonModal open={false} onClose={vi.fn()} />);
    expect(screen.queryByText(/Psiphon Outbounds/i)).toBeNull();
  });
});
