import { fireEvent, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import GeneralTab from '@/pages/settings/GeneralTab';
import { renderWithProviders } from './test-utils';

describe('GeneralTab', () => {
  it('keeps the stored page size when the field is cleared', () => {
    const updateSetting = vi.fn();

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings']}>
        <GeneralTab allSetting={new AllSetting({ pageSize: 25 })} updateSetting={updateSetting} />
      </MemoryRouter>,
    );

    const pageSizeInput = screen.getByDisplayValue('25');
    fireEvent.change(pageSizeInput, { target: { value: '' } });
    fireEvent.blur(pageSizeInput);

    expect(updateSetting).not.toHaveBeenCalled();
    expect((pageSizeInput as HTMLInputElement).value).toBe('25');
  });

  it('forwards typed page sizes unchanged, zero included', () => {
    const updateSetting = vi.fn();

    renderWithProviders(
      <MemoryRouter initialEntries={['/settings']}>
        <GeneralTab allSetting={new AllSetting({ pageSize: 25 })} updateSetting={updateSetting} />
      </MemoryRouter>,
    );

    fireEvent.change(screen.getByDisplayValue('25'), { target: { value: '0' } });

    expect(updateSetting).toHaveBeenCalledWith({ pageSize: 0 });
  });
});
