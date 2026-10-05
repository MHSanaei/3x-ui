import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import ClientSessionCell from '@/components/clients/ClientSessionCell';
import { renderWithProviders } from '@/test/test-utils';
import { IntlUtil } from '@/utils';

const START = 1735676400000;
const LAST_ONLINE = 1735680000000;

function openPopover(text: string) {
  fireEvent.mouseEnter(screen.getByText(text));
}

describe('ClientSessionCell', () => {
  it('shows a dash, not 0 B, for a client with no recorded session', () => {
    renderWithProviders(<ClientSessionCell lastOnline={LAST_ONLINE} />);

    expect(screen.getByText('—')).toBeTruthy();
    expect(screen.queryByText('0 B')).toBeNull();
  });

  it('shows an ongoing session with its start and no end', async () => {
    renderWithProviders(
      <ClientSessionCell up={1024} down={2048} start={START} lastOnline={LAST_ONLINE} ongoing />,
    );

    openPopover('3.00 KB');
    expect(await screen.findByText('Started')).toBeTruthy();
    expect(screen.getByText(IntlUtil.formatDate(START))).toBeTruthy();
    expect(screen.queryByText('Ended')).toBeNull();
  });

  it('shows when an ended session ended', async () => {
    renderWithProviders(
      <ClientSessionCell up={1024} down={2048} start={START} lastOnline={LAST_ONLINE} />,
    );

    openPopover('3.00 KB');
    expect(await screen.findByText('Ended')).toBeTruthy();
    expect(screen.getByText(IntlUtil.formatDate(LAST_ONLINE))).toBeTruthy();
  });
});
