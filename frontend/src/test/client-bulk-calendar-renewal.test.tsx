import { expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import ClientBulkAddModal from '@/pages/clients/ClientBulkAddModal';
import { HttpUtil, Msg } from '@/utils';
import { chooseSelectOption, renderWithProviders } from './test-utils';

const { bulkCreate } = vi.hoisted(() => ({ bulkCreate: vi.fn() }));
vi.mock('@/hooks/useClients', () => ({ useClients: () => ({ bulkCreate }) }));

it('keeps bulk renewal disabled by default and requires explicit cutoff selection without changing first-use duration', async () => {
  bulkCreate.mockResolvedValue(new Msg(true, '', { created: 1, skipped: [] }));
  const post = vi.spyOn(HttpUtil, 'post').mockImplementation(async (url, body) => {
    if (url !== '/panel/api/clients/renewalPreview')
      return new Msg(true, '', { datepicker: 'gregorian' });
    const request = body as { expiryTime: number };
    return new Msg(true, '', {
      timeZone: 'Asia/Taipei',
      renewAt: '',
      validThrough: '',
      nextExpiry: '',
      suggestedExpiryTime: 1893427200000,
      suggestedExpiry: '2030-01-01T00:00:00+08:00',
      renewals: 0,
      canRenew: false,
      delayedStart: request.expiryTime < 0,
    });
  });
  async function submit() {
    const button = document.querySelector('.ant-modal-footer .ant-btn-primary');
    if (!button) throw new Error('Create button missing');
    await waitFor(() => expect(button.classList.contains('ant-btn-loading')).toBe(false));
    fireEvent.click(button);
  }
  try {
    renderWithProviders(
      <ClientBulkAddModal
        open
        inbounds={[{ id: 1, protocol: 'vless', tag: 'calendar' }]}
        onOpenChange={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Select all' }));
    const mode = screen.getByLabelText('Auto renewal');
    expect(mode.closest('.ant-select')?.textContent).toContain('Disabled');
    expect(post.mock.calls.some(([url]) => url === '/panel/api/clients/renewalPreview')).toBe(
      false,
    );
    await submit();
    await waitFor(() =>
      expect(bulkCreate).toHaveBeenCalledWith([
        expect.objectContaining({
          client: expect.objectContaining({
            reset: 0,
            resetDay: 0,
            resetWeekday: 0,
            expiryTime: 0,
          }),
        }),
      ]),
    );
    chooseSelectOption(mode.id, 'Calendar weekly');
    await waitFor(() => expect(document.body.textContent).toContain('An expiry must be set'));
    await submit();
    await waitFor(() =>
      expect(bulkCreate).toHaveBeenLastCalledWith([
        expect.objectContaining({
          client: expect.objectContaining({
            reset: 0,
            resetDay: 0,
            resetWeekday: 1,
            expiryTime: 0,
          }),
        }),
      ]),
    );
    fireEvent.click(screen.getByRole('button', { name: /Set first cycle cutoff/ }));
    await submit();
    await waitFor(() =>
      expect(bulkCreate).toHaveBeenLastCalledWith([
        expect.objectContaining({
          client: expect.objectContaining({ resetWeekday: 1, expiryTime: 1893427200000 }),
        }),
      ]),
    );
    const label = Array.from(document.querySelectorAll('.ant-form-item-label label')).find(
      (el) => el.textContent === 'Start After First Use',
    );
    const toggle = label?.closest('.ant-form-item')?.querySelector('[role="switch"]');
    if (!toggle) throw new Error('First-use switch missing');
    fireEvent.click(toggle);
    const daysLabel = Array.from(document.querySelectorAll('.ant-form-item-label label')).find(
      (el) => el.textContent === 'Duration (days)',
    );
    const daysInput = daysLabel?.closest('.ant-form-item')?.querySelector('input');
    if (!daysInput) throw new Error('First-use days input missing');
    fireEvent.change(daysInput, { target: { value: '7' } });
    await waitFor(() =>
      expect(document.body.textContent).toContain('Dates are available after first-use activation'),
    );
    expect(screen.queryByRole('button', { name: /Set first cycle cutoff/ })).toBeNull();
    await submit();
    await waitFor(() =>
      expect(bulkCreate).toHaveBeenLastCalledWith([
        expect.objectContaining({
          client: expect.objectContaining({ resetWeekday: 1, expiryTime: -604800000 }),
        }),
      ]),
    );
  } finally {
    post.mockRestore();
  }
});
