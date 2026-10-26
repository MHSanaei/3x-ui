import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';

import ClientBulkAdjustModal from '@/pages/clients/ClientBulkAdjustModal';
import { renderWithProviders } from './test-utils';

async function submit() {
  const button = screen.getByRole('button', { name: /Confirm$/ });
  await waitFor(() => expect(button.classList.contains('ant-btn-loading')).toBe(false));
  await act(async () => {
    fireEvent.click(button);
  });
}

it('appends both link types with metadata and reports duplicates without sending hidden quota changes', async () => {
  const onSubmit = vi.fn().mockResolvedValue({
    adjusted: 2,
    externalLinks: { added: 3, skipped: 1, errors: 0 },
  });
  const onOpenChange = vi.fn();
  renderWithProviders(
    <ClientBulkAdjustModal open count={2} onSubmit={onSubmit} onOpenChange={onOpenChange} />,
  );
  fireEvent.change(screen.getByLabelText('Add days'), { target: { value: '30' } });
  fireEvent.click(screen.getByRole('tab', { name: 'Append external links' }));
  expect(screen.getByRole('dialog').textContent).toContain('2');
  fireEvent.click(screen.getByRole('button', { name: 'Add External Link' }));
  fireEvent.change(
    screen.getByPlaceholderText(
      'vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://',
    ),
    { target: { value: 'vless://uuid@example.com:443' } },
  );
  fireEvent.click(screen.getByRole('button', { name: 'Add External Subscription' }));
  fireEvent.change(screen.getByPlaceholderText('https://provider.example/sub/…'), {
    target: { value: 'ftp://invalid.example/sub' },
  });
  await submit();
  await waitFor(() => expect(screen.getByText('Enter an http(s) subscription URL.')).toBeTruthy());
  expect(onSubmit).not.toHaveBeenCalled();
  fireEvent.change(screen.getByPlaceholderText('https://provider.example/sub/…'), {
    target: { value: ' https://provider.example/sub ' },
  });
  fireEvent.change(screen.getByLabelText('Name prefix'), { target: { value: '[backup] ' } });
  fireEvent.click(screen.getAllByRole('switch', { name: 'Enabled' })[1]);
  await submit();
  await waitFor(() =>
    expect(onSubmit).toHaveBeenCalledWith(0, 0, '', null, '', [
      {
        kind: 'link',
        value: 'vless://uuid@example.com:443',
        remark: '',
        enable: true,
        expiryTime: 0,
        namePrefix: '',
      },
      {
        kind: 'subscription',
        value: 'https://provider.example/sub',
        remark: '',
        enable: false,
        expiryTime: 0,
        namePrefix: '[backup] ',
      },
    ]),
  );
  await waitFor(() =>
    expect(
      screen.getByText('2 clients updated: 3 added, 1 duplicates skipped, 0 clients not found.'),
    ).toBeTruthy(),
  );
  expect(onOpenChange).toHaveBeenCalledWith(false);
});

it('preserves existing quota adjustment submission and keeps a failed append open', async () => {
  const onSubmit = vi.fn().mockResolvedValue(null);
  const onOpenChange = vi.fn();
  renderWithProviders(
    <ClientBulkAdjustModal open count={3} onSubmit={onSubmit} onOpenChange={onOpenChange} />,
  );
  fireEvent.change(screen.getByLabelText('Add days'), { target: { value: '-7' } });
  await submit();
  await waitFor(() => expect(onSubmit).toHaveBeenCalledWith(-7, 0, '', null, '', undefined));
  expect(onOpenChange).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('tab', { name: 'Append external links' }));
  fireEvent.click(screen.getByRole('button', { name: 'Add External Link' }));
  fireEvent.change(
    screen.getByPlaceholderText(
      'vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://',
    ),
    { target: { value: 'not-a-link' } },
  );
  await submit();
  await waitFor(() => expect(screen.getByText('Enter a supported share link.')).toBeTruthy());
  expect(onSubmit).toHaveBeenCalledTimes(1);
  fireEvent.change(
    screen.getByPlaceholderText(
      'vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://',
    ),
    { target: { value: 'trojan://pw@example.com:443' } },
  );
  await submit();
  await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(2));
  expect(onOpenChange).not.toHaveBeenCalled();
});
