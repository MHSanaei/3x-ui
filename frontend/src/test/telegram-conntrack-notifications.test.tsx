import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { TelegramNotifications } from '@/components/ui/notifications/TelegramNotifications';
import { AllSetting } from '@/models/setting';
import { AllSettingSchema } from '@/schemas/setting';

import { renderWithProviders } from './test-utils';

describe('Telegram conntrack notifications', () => {
  it('offers a configurable threshold like the CPU notification', () => {
    const settings = new AllSetting({ tgEnabledEvents: 'conntrack.high', tgConntrack: 75 });
    const updateSetting = vi.fn();
    renderWithProviders(
      <TelegramNotifications allSetting={settings} updateSetting={updateSetting} />,
    );

    const high = screen.getByRole('checkbox', { name: 'Conntrack usage (%)' });
    expect((high as HTMLInputElement).checked).toBe(true);
    expect(screen.queryByRole('checkbox', { name: /Conntrack recovery/ })).toBeNull();
    const threshold = screen.getByRole('spinbutton', { name: 'Conntrack usage (%)' });
    expect((threshold as HTMLInputElement).value).toBe('75');
    fireEvent.change(threshold, { target: { value: '90' } });
    expect(updateSetting).toHaveBeenLastCalledWith({ tgConntrack: 90 });
    fireEvent.click(high);
    expect(updateSetting).toHaveBeenLastCalledWith({ tgEnabledEvents: '' });
  });

  it('validates the threshold between 0 and 100', () => {
    const schema = AllSettingSchema.shape.tgConntrack;
    expect(schema.safeParse(0).success).toBe(true);
    expect(schema.safeParse(100).success).toBe(true);
    expect(schema.safeParse(-1).success).toBe(false);
    expect(schema.safeParse(101).success).toBe(false);
    expect(schema.safeParse(80.5).success).toBe(false);
  });

  it('defaults the threshold to 80 and normalizes saved values', () => {
    expect(new AllSetting().tgConntrack).toBe(80);
    expect(new AllSetting({ tgConntrack: 0 }).tgConntrack).toBe(0);
    expect(new AllSetting({ tgConntrack: 150 }).tgConntrack).toBe(100);
    expect(new AllSetting({ tgConntrack: -10 }).tgConntrack).toBe(0);
    expect(new AllSetting({ tgConntrack: Number.NaN }).tgConntrack).toBe(80);
  });
});
