import { act, fireEvent, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import GeneralTab from '@/pages/settings/GeneralTab';
import { AllSettingSchema } from '@/schemas/setting';
import { renderWithProviders } from './test-utils';

// Mounting fetches inbound options with no visible change; settle it inside act().
async function renderGeneralTab(
  updateSetting: (patch: Partial<AllSetting>) => void,
  allSetting = new AllSetting({ pageSize: 25 }),
) {
  renderWithProviders(
    <MemoryRouter initialEntries={['/settings']}>
      <GeneralTab allSetting={allSetting} updateSetting={updateSetting} />
    </MemoryRouter>,
  );
  await act(async () => {});
}

describe('GeneralTab', () => {
  it('defaults the TCP connection limit to disabled and forwards positive values', async () => {
    const updateSetting = vi.fn();
    expect(new AllSetting().tcpPerIpConnectionLimit).toBe(0);

    await renderGeneralTab(updateSetting);

    const input = screen.getByRole('spinbutton', { name: 'TCP connections per IP' });
    expect((input as HTMLInputElement).value).toBe('0');
    fireEvent.change(input, { target: { value: '200' } });
    expect(updateSetting).toHaveBeenLastCalledWith({ tcpPerIpConnectionLimit: 200 });
  });

  it('allows disabling a configured TCP connection limit with zero', async () => {
    const updateSetting = vi.fn();
    await renderGeneralTab(updateSetting, new AllSetting({ tcpPerIpConnectionLimit: 200 }));

    fireEvent.change(screen.getByRole('spinbutton', { name: 'TCP connections per IP' }), {
      target: { value: '0' },
    });
    expect(updateSetting).toHaveBeenCalledWith({ tcpPerIpConnectionLimit: 0 });
  });

  it('validates the TCP connection limit before settings can be saved', () => {
    for (const value of [0, 200, 1000000]) {
      expect(AllSettingSchema.safeParse({ tcpPerIpConnectionLimit: value }).success).toBe(true);
    }
    for (const value of [-1, 1.5, 1000001, '200']) {
      expect(AllSettingSchema.safeParse({ tcpPerIpConnectionLimit: value }).success).toBe(false);
    }
  });

  it('keeps the stored page size when the field is cleared', async () => {
    const updateSetting = vi.fn();

    await renderGeneralTab(updateSetting);

    const pageSizeInput = screen.getByDisplayValue('25');
    fireEvent.change(pageSizeInput, { target: { value: '' } });
    fireEvent.blur(pageSizeInput);

    expect(updateSetting).not.toHaveBeenCalled();
    expect((pageSizeInput as HTMLInputElement).value).toBe('25');
  });

  it('forwards typed page sizes unchanged, zero included', async () => {
    const updateSetting = vi.fn();

    await renderGeneralTab(updateSetting);

    fireEvent.change(screen.getByDisplayValue('25'), { target: { value: '0' } });

    expect(updateSetting).toHaveBeenCalledWith({ pageSize: 0 });
  });
});
