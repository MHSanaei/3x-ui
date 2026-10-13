import { expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';

import ClientFormModal from '@/pages/clients/ClientFormModal';
import { chooseSelectOption, renderWithProviders } from './test-utils';

it('shows only the active renewal mode guidance when switching between interval, weekly, monthly, and disabled', async () => {
  renderWithProviders(
    <ClientFormModal
      open
      mode="edit"
      client={{ email: 'layout@example.com', reset: 31, expiryTime: 1893427200000 }}
      attachedIds={[1]}
      inbounds={[{ id: 1, protocol: 'vless', tag: 'calendar' }]}
      save={async () => null}
      onOpenChange={() => {}}
    />,
  );
  const mode = screen.getByLabelText('Auto renewal');
  const interval = /Adds the configured number of 24-hour days/;
  const calendar = /Calendar renewal uses midnight/;
  const monthly = /Monthly day 1 means valid through/;

  await waitFor(() => expect(screen.queryByText(calendar)).toBeNull());
  expect(screen.queryByText(interval)).not.toBeNull();
  expect(screen.queryByText(monthly)).toBeNull();

  chooseSelectOption(mode.id, 'Calendar weekly');
  expect(screen.queryByText(calendar)).not.toBeNull();
  expect(screen.queryByText(interval)).toBeNull();
  expect(screen.queryByText(monthly)).toBeNull();

  chooseSelectOption(mode.id, 'Calendar monthly');
  expect(screen.queryByText(calendar)).not.toBeNull();
  expect(screen.queryByText(monthly)).not.toBeNull();
  expect(screen.queryByText(interval)).toBeNull();

  chooseSelectOption(mode.id, 'Disabled');
  expect(screen.queryByText(calendar)).toBeNull();
  expect(screen.queryByText(interval)).toBeNull();
  expect(screen.queryByText(monthly)).toBeNull();
});
