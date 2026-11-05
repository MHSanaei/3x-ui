import { describe, expect, it } from 'vitest';
import { useState } from 'react';
import { ConfigProvider } from 'antd';
import userEvent from '@testing-library/user-event';
import '@testing-library/jest-dom/vitest';
import { TunnelFormSchema, TunnelViewSchema, defaultTunnel } from '@/schemas/tunnel';

describe('tunnel wire contracts', () => {
  it('keeps config and observed status nested in the API response', () => {
    const values = defaultTunnel();
    const view = TunnelViewSchema.parse({
      id: 1,
      guid: 'test',
      name: 'edge',
      revision: 1,
      nodeId: null,
      enabled: false,
      managed: false,
      config: values.config,
      status: { state: 'stopped', carrier: '', reason: '', revision: 1, metrics: null },
    });
    expect(view.config.carrier).toBe('tcp');
    expect(view.status.state).toBe('stopped');
    expect(TunnelViewSchema.safeParse({ ...values.config, id: 1, state: 'stopped' }).success).toBe(
      false,
    );
  });
  it('rejects invalid forwarding ports and permits TCP/UDP on the same number', () => {
    const values = defaultTunnel();
    values.name = 'edge';
    values.config.endpoint = 'example.com';
    values.config.ports[0].listen = 65536;
    const bad = TunnelFormSchema.safeParse(values);
    expect(bad.success).toBe(false);
    if (!bad.success) expect(bad.error.issues[0].path).toEqual(['config', 'ports', 0, 'listen']);
    values.config.ports[0].listen = 8443;
    values.config.ports.push({ listen: 8443, target: 443, protocol: 'udp' });
    expect(TunnelFormSchema.parse(values).config.ports.map((p) => p.protocol)).toEqual([
      'tcp',
      'udp',
    ]);
  });
});

import { fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import TunnelForm from '@/pages/tunnels/TunnelForm';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

afterEach(() => vi.restoreAllMocks());

function mockTunnelRequests() {
  vi.spyOn(HttpUtil, 'get').mockImplementation(
    async (url) =>
      new Msg(
        true,
        '',
        url.includes('capabilities')
          ? {
              supported: true,
              os: 'windows',
              arch: 'amd64',
              version: '3.3.0',
              carriers: ['tcp', 'udp', 'tls', 'wss'],
              rawSocket: false,
              error: '',
            }
          : [],
      ),
  );
}

it('submits a nested edited configuration without revealing or replacing its key', async () => {
  mockTunnelRequests();
  const defaults = defaultTunnel();
  defaults.config.endpoint = 'exit.example.com';
  const record = TunnelViewSchema.parse({
    id: 1,
    guid: 'guid',
    name: 'edge',
    nodeId: null,
    revision: 4,
    enabled: false,
    managed: false,
    config: defaults.config,
    status: { state: 'stopped', carrier: '', reason: '', revision: 4, metrics: null },
  });
  const save = vi.fn(async () => {});
  renderWithProviders(<TunnelForm open record={record} onClose={() => {}} onSave={save} />);
  const name = await screen.findByLabelText('Remark');
  fireEvent.change(name, { target: { value: 'renamed tunnel' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save' }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
  const data = save.mock.calls[0] as unknown as [ReturnType<typeof defaultTunnel>];
  expect(data[0].name).toBe('renamed tunnel');
  expect(data[0].config.endpoint).toBe('exit.example.com');
  expect(data[0].revision).toBe(4);
  expect(data[0].secret).toBe('');
  expect(data[0].generateKey).toBe(false);
  expect((screen.getByRole('option', { name: 'ICMP' }) as HTMLOptionElement).disabled).toBe(true);
});

it('keeps keyboard focus in the dialog, focuses invalid fields, and restores the trigger', async () => {
  mockTunnelRequests();
  const save = vi.fn(async () => {});
  function Harness() {
    const [open, setOpen] = useState(false);
    return (
      <ConfigProvider theme={{ token: { motion: false } }}>
        <button onClick={() => setOpen(true)}>Open tunnels</button>
        <TunnelForm open={open} record={null} onClose={() => setOpen(false)} onSave={save} />
      </ConfigProvider>
    );
  }
  renderWithProviders(<Harness />);
  const user = userEvent.setup();
  const trigger = screen.getByRole('button', { name: 'Open tunnels' });
  await user.tab();
  expect(trigger).toHaveFocus();
  await user.keyboard('{Enter}');
  const dialog = await screen.findByRole('dialog');
  const tabTo = async (target: HTMLElement) => {
    for (let i = 0; i < 60 && document.activeElement !== target; i++) {
      await user.tab();
      expect(dialog).toContainElement(document.activeElement as HTMLElement);
    }
    expect(target).toHaveFocus();
  };
  const name = screen.getByLabelText('Remark');
  await tabTo(name);
  await user.tab();
  expect(screen.getByRole('combobox', { name: 'Nodes' })).toHaveFocus();
  await tabTo(screen.getByRole('button', { name: 'Save' }));
  await user.keyboard('{Enter}');
  await waitFor(() => expect(name).toHaveFocus());
  expect(name).toHaveAttribute('aria-invalid', 'true');
  expect(name).toHaveAccessibleDescription();
  expect(save).not.toHaveBeenCalled();
  await user.keyboard('{Escape}');
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  await waitFor(() => expect(trigger).toHaveFocus());
});
