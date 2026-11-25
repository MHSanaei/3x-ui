import { describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConfigProvider } from 'antd';

import InboundFormModal from '@/pages/inbounds/form/InboundFormModal';
import { DBInbound } from '@/models/dbinbound';
import { HttpUtil } from '@/utils';

import { renderWithProviders } from './test-utils';

function renderInbound(mode: 'add' | 'edit' = 'add') {
  const inbound = new DBInbound({
    id: 41,
    protocol: 'vless',
    enable: true,
    port: 14443,
    settings: { clients: [], decryption: 'none', encryption: 'none' },
    streamSettings: { network: 'tcp', security: 'none', tcpSettings: {} },
  });
  renderWithProviders(
    <ConfigProvider theme={{ token: { motion: false } }}>
      <InboundFormModal
        open
        mode={mode}
        dbInbound={mode === 'edit' ? inbound : null}
        dbInbounds={[]}
        availableNodes={[]}
        onClose={() => {}}
        onSaved={() => {}}
      />
    </ConfigProvider>,
  );
  return userEvent.setup();
}

describe('Inbound screen-reader interaction', () => {
  it('exposes the selected protocol as the closed control value', () => {
    renderInbound();
    expect(screen.getByRole('combobox', { name: 'Protocol' })).toHaveDisplayValue('vless');
  });
  it('exposes all visible protocol options as selectable items within its dialog', async () => {
    const user = renderInbound();
    const dialog = screen.getByRole('dialog', { name: 'Add Inbound' });
    const protocol = within(dialog).getByRole('combobox', { name: 'Protocol' });
    const shadowsocks = within(protocol).getByRole('option', { name: 'shadowsocks' });
    expect(within(protocol).getByRole('option', { name: 'tuic' })).toBeInTheDocument();
    expect(
      within(protocol).getByRole('option', { name: 'vless', selected: true }),
    ).toBeInTheDocument();
    await user.selectOptions(protocol, shadowsocks);
    expect(protocol).toHaveDisplayValue('shadowsocks');
    await user.click(screen.getByRole('tab', { name: 'Protocol' }));
    expect(screen.getByRole('combobox', { name: 'Encryption method' })).toBeVisible();
  });

  it('selects transport by its option role and submits the actual create payload', async () => {
    const post = vi.mocked(HttpUtil.post).mockClear();
    const user = renderInbound();
    await user.click(screen.getByRole('tab', { name: 'Stream' }));
    const transport = screen.getByRole('combobox', { name: 'Transmission' });
    await user.selectOptions(
      transport,
      within(transport).getByRole('option', { name: 'WebSocket' }),
    );
    const path = screen.getByRole('textbox', { name: 'Path' });
    await user.clear(path);
    await user.type(path, '/accessible');
    await user.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/panel/api/inbounds/add', expect.anything()),
    );
    const payload = post.mock.calls.find(
      ([url]) => url === '/panel/api/inbounds/add',
    )![1] as Record<string, unknown>;
    expect(JSON.parse(String(payload.streamSettings))).toMatchObject({
      network: 'ws',
      security: 'none',
      wsSettings: { path: '/accessible' },
    });
  });

  it('preserves the disabled protocol in edit mode and exposes a named security group', async () => {
    const user = renderInbound('edit');
    expect(screen.getByRole('combobox', { name: 'Protocol' })).toBeDisabled();
    await user.click(screen.getByRole('tab', { name: 'Security' }));
    const security = screen.getByRole('radiogroup', { name: 'Security' });
    expect(
      within(security)
        .getAllByRole('radio')
        .map((radio) => (radio as HTMLInputElement).value),
    ).toEqual(['none', 'tls', 'reality']);
  });

  it('preserves edit serialization after a keyboard-accessible transport change', async () => {
    const post = vi.mocked(HttpUtil.post).mockClear();
    const user = renderInbound('edit');
    await user.click(screen.getByRole('tab', { name: 'Stream' }));
    const transport = screen.getByRole('combobox', { name: 'Transmission' });
    await user.selectOptions(transport, within(transport).getByRole('option', { name: 'gRPC' }));
    await user.click(screen.getByRole('button', { name: 'Save Changes' }));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/panel/api/inbounds/update/41', expect.anything()),
    );
    const payload = post.mock.calls.find(
      ([url]) => url === '/panel/api/inbounds/update/41',
    )![1] as Record<string, unknown>;
    expect(payload).toMatchObject({ protocol: 'vless', port: 14443, enable: true });
    expect(JSON.parse(String(payload.streamSettings))).toMatchObject({
      network: 'grpc',
      security: 'none',
      grpcSettings: { serviceName: '' },
    });
    expect(JSON.parse(String(payload.settings))).not.toHaveProperty('clients');
  });

  it('reveals and describes a validation error on a hidden tab without submitting', async () => {
    const post = vi.mocked(HttpUtil.post).mockClear();
    const user = renderInbound();
    const strategy = screen.getByRole('combobox', { name: 'Share address strategy' });
    await user.selectOptions(strategy, within(strategy).getByRole('option', { name: 'Custom' }));
    const address = screen.getByRole('textbox', { name: 'Custom share address' });
    await user.type(address, 'https://example.test/path');
    await user.click(screen.getByRole('tab', { name: 'Stream' }));
    await user.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(address).toBeVisible());
    expect(address).toHaveAttribute('aria-invalid', 'true');
    expect(address).toHaveAccessibleDescription(expect.stringContaining('without a scheme'));
    await waitFor(() => expect(address).toHaveFocus());
    expect(post.mock.calls.filter(([url]) => url === '/panel/api/inbounds/add')).toEqual([]);
  });
});
