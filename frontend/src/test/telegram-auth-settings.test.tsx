import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import TelegramAuthSettings from '@/pages/settings/TelegramAuthSettings';
import { HttpUtil, Msg } from '@/utils';

afterEach(() => vi.restoreAllMocks());

describe('Telegram admin login settings', () => {
  it('verifies credentials and shows the bot link and command', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      new Msg(true, '', {
        available: true,
        linked: false,
        telegramUserId: 0,
        botUsername: 'MyAdminBot',
      }),
    );
    const post = vi
      .spyOn(HttpUtil, 'post')
      .mockResolvedValue(new Msg(true, '', { code: 'ABC123', expiresAt: Date.now() + 300_000 }));

    render(<TelegramAuthSettings twoFactorEnabled />);
    fireEvent.click(await screen.findByRole('button', { name: 'Link account' }));
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'password' } });
    fireEvent.change(screen.getByLabelText('Code'), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }));

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/panel/api/setting/telegramAuth/link', {
        password: 'password',
        twoFactorCode: '123456',
      }),
    );
    expect(await screen.findByText('/link ABC123')).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Open bot in Telegram' }).getAttribute('href')).toBe(
      'https://t.me/MyAdminBot?start=link_ABC123',
    );
  });

  it('allows unlinking when the bot is unavailable', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      new Msg(true, '', { available: false, linked: true, telegramUserId: 42 }),
    );

    render(<TelegramAuthSettings twoFactorEnabled={false} />);
    expect(await screen.findByRole('button', { name: 'Unlink account' })).toBeTruthy();
  });
});
