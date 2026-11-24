import { fireEvent, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { HostRecord } from '@/api/queries/useHostsQuery';
import HostFormModal from '@/pages/hosts/HostFormModal';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

const host = {
  groupId: 'g1',
  remark: 'DE WS',
  inboundIds: [1],
  hosts: ['a.example.com:8443', 'b.example.com:2096'],
  port: 8443,
  security: 'same',
} as HostRecord;

describe('Host form port', () => {
  beforeEach(() => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', []));
  });

  it('rewrites every address to the new port when the Port field changes', async () => {
    const save = vi.fn().mockResolvedValue({ success: true });
    renderWithProviders(
      <HostFormModal
        open
        mode="edit"
        host={host}
        inboundOptions={[{ id: 1, remark: 'in', tag: 'in', protocol: 'vless' }]}
        existingHosts={[host]}
        save={save}
        onOpenChange={() => {}}
      />,
    );
    const port = await screen.findByRole('spinbutton');
    fireEvent.change(port, { target: { value: '9443' } });
    fireEvent.blur(port);

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0][0]).toMatchObject({
      port: 9443,
      hosts: ['a.example.com:9443', 'b.example.com:9443'],
    });
  });
});
