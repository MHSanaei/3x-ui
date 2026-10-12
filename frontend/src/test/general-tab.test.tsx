import { act, fireEvent, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import GeneralTab from '@/pages/settings/GeneralTab';
import { renderWithProviders } from './test-utils';

// Mounting fetches inbound options with no visible change; settle it inside act().
async function renderGeneralTab(updateSetting: (patch: Partial<AllSetting>) => void) {
  renderWithProviders(
    <MemoryRouter initialEntries={['/settings']}>
      <GeneralTab allSetting={new AllSetting({ pageSize: 25 })} updateSetting={updateSetting} />
    </MemoryRouter>,
  );
  await act(async () => {});
}

describe('GeneralTab', () => {
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
