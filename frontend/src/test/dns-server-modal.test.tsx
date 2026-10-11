import { describe, expect, it, vi } from 'vitest';
import { act, fireEvent } from '@testing-library/react';

import DnsServerModal from '@/pages/xray/dns/DnsServerModal';
import { renderWithProviders } from './test-utils';

describe('DnsServerModal', () => {
  // A Lua dns.script (xray-core 26.10.10) picks servers by id; the modal rebuilds the
  // server on save, and collapsing an otherwise plain one to its address lost the id.
  it('keeps the server id through an edit', async () => {
    const onConfirm = vi.fn();
    renderWithProviders(
      <DnsServerModal
        open
        server={{ address: '1.1.1.1', id: 'cf', timeoutMs: 4000 }}
        isEdit
        onClose={() => {}}
        onConfirm={onConfirm}
      />,
    );

    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await act(async () => {
      fireEvent.click(ok);
    });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm.mock.calls[0][0]).toMatchObject({ address: '1.1.1.1', id: 'cf' });
  });
});
