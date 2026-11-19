// @vitest-environment jsdom
import { useState } from 'react';
import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { AllSetting } from '@/models/setting';
import { EmailNotifications } from '@/components/ui/notifications/EmailNotifications';
import { AllSettingSchema } from '@/schemas/setting';
import { renderWithProviders } from './test-utils';

function EditableNotifications() {
  const [settings, setSettings] = useState(new AllSetting({ smtpEnabledEvents: 'cpu.high' }));
  return (
    <>
      <EmailNotifications
        allSetting={settings}
        updateSetting={(patch) => setSettings((old) => new AllSetting({ ...old, ...patch }))}
      />
      <output data-testid="settings">{JSON.stringify(settings)}</output>
    </>
  );
}

describe('email user traffic alerts', () => {
  it('adds both rules beside CPU and persists custom thresholds without dropping CPU', () => {
    renderWithProviders(<EditableNotifications />);
    fireEvent.click(screen.getByRole('checkbox', { name: 'User sustained high bandwidth' }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'User rolling-window traffic' }));
    const rate = screen.getByRole('spinbutton', { name: 'Rate threshold (MB/s)' });
    const duration = screen.getByRole('spinbutton', { name: 'Continuous duration (seconds)' });
    const traffic = screen.getByRole('spinbutton', { name: 'Traffic threshold (GB)' });
    const window = screen.getByRole('spinbutton', { name: 'Rolling window (minutes)' });
    expect(rate.getAttribute('value')).toBe('7');
    expect(duration.getAttribute('value')).toBe('60');
    expect(traffic.getAttribute('value')).toBe('5');
    expect(window.getAttribute('value')).toBe('30');
    fireEvent.change(rate, { target: { value: '9' } });
    fireEvent.blur(rate);
    fireEvent.change(window, { target: { value: '45' } });
    fireEvent.blur(window);
    const saved = JSON.parse(screen.getByTestId('settings').textContent!);
    expect(saved.smtpEnabledEvents.split(',')).toEqual([
      'cpu.high',
      'user.rate.high',
      'user.traffic.high',
    ]);
    expect(saved.smtpUserRateMB).toBe(9);
    expect(saved.smtpUserWindowMinutes).toBe(45);
  });

  it('validates bounded positive thresholds', () => {
    expect(
      AllSettingSchema.safeParse({ smtpUserRateMB: 7, smtpUserWindowMinutes: 30 }).success,
    ).toBe(true);
    expect(AllSettingSchema.safeParse({ smtpUserRateMB: 0 }).success).toBe(false);
    expect(AllSettingSchema.safeParse({ smtpUserWindowMinutes: 1441 }).success).toBe(false);
  });
});
