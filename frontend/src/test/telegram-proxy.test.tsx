import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import TelegramTab from '@/pages/settings/TelegramTab';

describe('TelegramTab proxy settings', () => {
  it('keeps SOCKS5 selected before a proxy host is entered', async () => {
    const allSetting = new AllSetting();
    const updateSetting = vi.fn();
    const { container } = render(<TelegramTab allSetting={allSetting} updateSetting={updateSetting} />);

    const selector = container.querySelector('.ant-select-content');
    expect(selector).not.toBeNull();
    fireEvent.mouseDown(selector as Element);

    fireEvent.click(await screen.findByTitle('SOCKS5'));

    expect(container.querySelector('.ant-select-content[title="SOCKS5"]')).toBeTruthy();
    expect(screen.getByPlaceholderText('127.0.0.1')).toBeTruthy();
    expect(updateSetting).not.toHaveBeenCalledWith({ tgBotProxy: '' });
  });
});
