import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { TelegramNotifications } from '@/components/ui/notifications/TelegramNotifications';
import { AllSetting } from '@/models/setting';

import { renderWithProviders } from './test-utils';

describe('Telegram conntrack notifications', () => {
  it('offers independent high-usage and recovery event selections', () => {
    const settings = new AllSetting({ tgEnabledEvents: 'conntrack.high' });
    const updateSetting = vi.fn();
    renderWithProviders(
      <TelegramNotifications allSetting={settings} updateSetting={updateSetting} />,
    );

    const high = screen.getByRole('checkbox', { name: 'Conntrack usage (80% or more)' });
    const recovered = screen.getByRole('checkbox', { name: 'Conntrack recovery (below 70%)' });
    expect((high as HTMLInputElement).checked).toBe(true);
    expect((recovered as HTMLInputElement).checked).toBe(false);

    fireEvent.click(recovered);
    expect(updateSetting).toHaveBeenLastCalledWith({
      tgEnabledEvents: 'conntrack.high,conntrack.recovered',
    });
    fireEvent.click(high);
    expect(updateSetting).toHaveBeenLastCalledWith({ tgEnabledEvents: '' });
  });
});
