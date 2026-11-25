import { useState } from 'react';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import WechatTab from '@/pages/settings/WechatTab';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

function StatefulTab({ initial = new AllSetting() }: { initial?: AllSetting }) {
  const [setting, setSetting] = useState(initial);
  return (
    <>
      <WechatTab
        allSetting={setting}
        hasPendingChanges={!setting.equals(initial)}
        updateSetting={(patch) => setSetting((current) => new AllSetting({ ...current, ...patch }))}
      />
      <output data-testid="draft">{JSON.stringify(setting)}</output>
    </>
  );
}

describe('WeChat notification settings', () => {
  it('retains saved webhook credentials until an explicit clear is saved', () => {
    renderWithProviders(
      <StatefulTab initial={new AllSetting({ wechatEnable: true, hasWechatWebhookURL: true })} />,
    );
    const input = screen.getByLabelText('Webhook URL') as HTMLInputElement;
    expect(input.value).toBe('');
    expect(
      screen.getByText('A webhook URL is saved. Leave this field blank to keep it.'),
    ).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }));
    expect(JSON.parse(screen.getByTestId('draft').textContent || '{}').clearWechatWebhookURL).toBe(
      true,
    );
    fireEvent.change(input, { target: { value: 'https://example.com/notify' } });
    const draft = JSON.parse(screen.getByTestId('draft').textContent || '{}');
    expect(draft.wechatWebhookURL).toBe('https://example.com/notify');
    expect(draft.clearWechatWebhookURL).toBe(false);
  });

  it('shows optional bearer authentication only for a compatible custom gateway', async () => {
    renderWithProviders(<StatefulTab />);
    expect(screen.queryByLabelText('Bearer token')).toBeNull();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Delivery mode' }));
    fireEvent.click(await screen.findByText('Custom webhook gateway'));
    expect(screen.getByLabelText('Bearer token')).toBeTruthy();
    expect(screen.getByText(/Personal WeChat requires a compatible gateway/)).toBeTruthy();
  });

  it('keeps scheduled reports independent from event selection', async () => {
    renderWithProviders(<StatefulTab />);
    fireEvent.click(screen.getByRole('tab', { name: /Notifications/ }));
    const reportToggle = screen.getByRole('switch', { name: 'Scheduled reports' });
    expect(reportToggle.getAttribute('aria-checked')).toBe('false');
    expect(screen.queryByRole('combobox', { name: 'Report schedule' })).toBeNull();
    fireEvent.click(reportToggle);
    expect(screen.getByRole('combobox', { name: 'Report schedule' })).toBeTruthy();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Crash' }));
    await waitFor(() => {
      const draft = JSON.parse(screen.getByTestId('draft').textContent || '{}');
      expect(draft.wechatReportEnable).toBe(true);
      expect(draft.wechatEnabledEvents).toBe('login.attempt,cpu.high,xray.crash');
    });
  });

  it('tests the saved server configuration and shows the actual delivery result', async () => {
    const post = vi
      .spyOn(HttpUtil, 'post')
      .mockResolvedValueOnce(new Msg(false, 'Destination rejected the message'));
    renderWithProviders(
      <StatefulTab initial={new AllSetting({ wechatEnable: true, hasWechatWebhookURL: true })} />,
    );
    expect(
      screen.getByText('Save changes before testing. The test uses the saved configuration.'),
    ).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Send test notification' }));
    expect(await screen.findByText('Destination rejected the message')).toBeTruthy();
    expect(post.mock.calls.some(([path]) => path === '/panel/api/setting/testWechat')).toBe(true);
  });

  it('updates WeChat thresholds without changing another notification provider', () => {
    renderWithProviders(<StatefulTab initial={new AllSetting({ wechatCpu: 80, smtpCpu: 70 })} />);
    fireEvent.click(screen.getByRole('tab', { name: /Notifications/ }));
    fireEvent.change(screen.getByRole('spinbutton', { name: 'CPU high (%)' }), {
      target: { value: '92' },
    });
    const draft = JSON.parse(screen.getByTestId('draft').textContent || '{}');
    expect(draft.wechatCpu).toBe(92);
    expect(draft.smtpCpu).toBe(70);
    expect(draft.wechatEnabledEvents).toBe('login.attempt,cpu.high');
    expect(draft.wechatReportEnable).toBe(false);
  });

  it('shows a successful saved-configuration test when the response has no detail', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValueOnce(new Msg(true));
    renderWithProviders(<StatefulTab initial={new AllSetting({ wechatEnable: true })} />);
    fireEvent.click(screen.getByRole('button', { name: 'Send test notification' }));
    expect(await screen.findByText('Test notification sent successfully')).toBeTruthy();
    expect(screen.queryByText('Request failed')).toBeNull();
  });

  it('does not test a disabled channel or a destination with unsaved changes', () => {
    const post = vi.spyOn(HttpUtil, 'post');
    const callsBefore = post.mock.calls.length;
    const { unmount } = renderWithProviders(<StatefulTab />);
    let button = screen.getByRole('button', {
      name: 'Send test notification',
    }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    fireEvent.click(button);
    expect(post.mock.calls.length).toBe(callsBefore);
    unmount();
    renderWithProviders(
      <StatefulTab initial={new AllSetting({ wechatEnable: true, hasWechatWebhookURL: true })} />,
    );
    button = screen.getByRole('button', { name: 'Send test notification' }) as HTMLButtonElement;
    expect(button.disabled).toBe(false);
    fireEvent.change(screen.getByLabelText('Webhook URL'), {
      target: { value: 'https://receiver.example/notify' },
    });
    expect(button.disabled).toBe(true);
    fireEvent.click(button);
    expect(post.mock.calls.length).toBe(callsBefore);
  });

  it('keeps the saved integer threshold when its input is cleared', () => {
    renderWithProviders(<StatefulTab initial={new AllSetting({ wechatCpu: 80 })} />);
    fireEvent.click(screen.getByRole('tab', { name: /Notifications/ }));
    fireEvent.change(screen.getByRole('spinbutton', { name: 'CPU high (%)' }), {
      target: { value: '' },
    });
    expect(JSON.parse(screen.getByTestId('draft').textContent || '{}').wechatCpu).toBe(80);
  });

  it('stores metric thresholds as valid integer percentages', () => {
    renderWithProviders(<StatefulTab initial={new AllSetting({ wechatCpu: 80 })} />);
    fireEvent.click(screen.getByRole('tab', { name: /Notifications/ }));
    const input = screen.getByRole('spinbutton', { name: 'CPU high (%)' });
    fireEvent.change(input, { target: { value: '92.5' } });
    fireEvent.blur(input);
    expect(JSON.parse(screen.getByTestId('draft').textContent || '{}').wechatCpu).toBe(93);
  });
});
