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
  hosts: ['cdn.example.com'],
  security: 'same',
  sni: 'dash.example.com',
  snis: ['dash.example.com'],
} as HostRecord;

describe('Host form SNI', () => {
  beforeEach(() => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', []));
  });

  it('saves an empty SNI after the last SNI tag is removed', async () => {
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
    const tag = await screen.findByTitle('dash.example.com');
    const remove = tag.querySelector('.ant-select-selection-item-remove');
    if (!remove) throw new Error('missing SNI tag remove button');
    fireEvent.click(remove);
    await waitFor(() => expect(screen.queryByTitle('dash.example.com')).toBeNull());

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0][0]).toMatchObject({ sni: '', snis: [] });
  });
});
