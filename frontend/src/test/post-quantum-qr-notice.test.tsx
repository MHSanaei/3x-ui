import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { isPostQuantumLink } from '@/lib/xray/inbound-link';
import QrPanel from '@/pages/inbounds/qr/QrPanel';
import SubConfigsTab from '@/pages/sub/SubConfigsTab';

const REALITY_LINK =
  'vless://22222222-3333-4444-9555-666666666666@example.test:443' +
  '?encryption=none&security=reality&support-x25519mlkem768=true';
const PQ_LINK = REALITY_LINK + '&pqv=' + 'A'.repeat(2603);
const HINT =
  'QR codes are hidden for links containing post-quantum keys because they can be too large or dense to scan reliably. Copy the link and import it from the clipboard in your client app.';

describe('post-quantum QR explanation', () => {
  it('restores the subscription QR action for an ordinary REALITY link', () => {
    render(<SubConfigsTab links={[REALITY_LINK]} onCopy={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'QR' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'QR code unavailable' })).toBeNull();
  });

  it('explains the restriction on the subscription page and keeps copying available', async () => {
    const onCopy = vi.fn();
    render(<SubConfigsTab links={[PQ_LINK]} onCopy={onCopy} />);
    expect(screen.queryByRole('button', { name: 'QR' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Copy' }));
    expect(onCopy).toHaveBeenCalledWith(PQ_LINK);
    fireEvent.click(screen.getByRole('button', { name: 'QR code unavailable' }));
    expect(await screen.findByText(HINT)).toBeTruthy();
  });

  it('explains the restriction inside the client and inbound QR panel', async () => {
    const { container } = render(<QrPanel value={PQ_LINK} showQr={!isPostQuantumLink(PQ_LINK)} />);
    expect(container.querySelector('.qr-panel-canvas')).toBeNull();
    expect(screen.getByRole('button', { name: 'Copy' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'QR code unavailable' }));
    expect(await screen.findByText(HINT)).toBeTruthy();
  });

  it('does not blame post-quantum keys when QR is disabled for config text', () => {
    render(<QrPanel value='{"outbounds":[]}' showQr={false} />);
    expect(screen.queryByRole('button', { name: 'QR code unavailable' })).toBeNull();
  });
});
