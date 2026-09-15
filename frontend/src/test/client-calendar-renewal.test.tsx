import { expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import ClientFormModal from '@/pages/clients/ClientFormModal';
import { HttpUtil, Msg } from '@/utils';
import { chooseSelectOption, renderWithProviders } from './test-utils';

it('preserves monthly clients, previews backend dates, and saves exclusive weekly or disabled modes', async () => {
  const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue(
    new Msg(true, '', {
      timeZone: 'Asia/Taipei',
      renewAt: '2030-01-01T00:00:00+08:00',
      validThrough: '2029-12-31T23:59:59+08:00',
      nextExpiry: '2030-02-01T00:00:00+08:00',
      suggestedExpiryTime: 1893427200000,
      suggestedExpiry: '2030-01-01T00:00:00+08:00',
      renewals: 1,
      canRenew: true,
      delayedStart: false,
    }),
  );
  const save = vi.fn().mockResolvedValue(new Msg(true, '', null));
  async function submit() {
    const button = document.querySelector('.ant-modal-footer .ant-btn-primary');
    if (!button) throw new Error('Save button missing');
    await waitFor(() => expect(button.classList.contains('ant-btn-loading')).toBe(false));
    fireEvent.click(button);
  }
  try {
    renderWithProviders(
      <ClientFormModal
        open
        mode="edit"
        client={{
          email: 'monthly@example.com',
          uuid: '11111111-1111-1111-1111-111111111111',
          subId: 'calendar-sub',
          enable: true,
          expiryTime: 1893427200000,
          resetDay: 1,
          reset: 7,
          resetMax: 3,
          traffic: { resetCount: 2 },
        }}
        attachedIds={[1]}
        inbounds={[{ id: 1, protocol: 'vless', tag: 'calendar' }]}
        save={save}
        onOpenChange={() => {}}
      />,
    );
    const mode = screen.getByLabelText('Auto renewal');
    expect(mode.closest('.ant-select')?.textContent).toContain('Calendar monthly');
    await waitFor(() => expect(document.body.textContent).toContain('2029-12-31T23:59:59+08:00'));
    expect(post).toHaveBeenCalledWith(
      '/panel/api/clients/renewalPreview',
      expect.objectContaining({ resetMax: 3, resetCount: 2 }),
      expect.anything(),
    );
    await submit();
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        expect.objectContaining({
          reset: 7,
          resetDay: 1,
          resetWeekday: 0,
          expiryTime: 1893427200000,
        }),
        expect.anything(),
      ),
    );
    chooseSelectOption(mode.id, 'Calendar weekly');
    const weekday = screen.getByLabelText('Renew on weekday');
    chooseSelectOption(weekday.id, 'Sunday');
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith(
        '/panel/api/clients/renewalPreview',
        expect.objectContaining({ reset: 0, resetDay: 0, resetWeekday: 7 }),
        expect.anything(),
      ),
    );
    await submit();
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        expect.objectContaining({
          reset: 0,
          resetDay: 0,
          resetWeekday: 7,
          expiryTime: 1893427200000,
        }),
        expect.anything(),
      ),
    );
    chooseSelectOption(mode.id, 'Disabled');
    await submit();
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        expect.objectContaining({
          reset: 0,
          resetDay: 0,
          resetWeekday: 0,
          expiryTime: 1893427200000,
        }),
        expect.anything(),
      ),
    );
  } finally {
    post.mockRestore();
  }
});
