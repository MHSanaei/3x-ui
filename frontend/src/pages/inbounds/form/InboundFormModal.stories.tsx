import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within, waitFor } from 'storybook/test';
import { Button } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { AntdRootConfig } from '@/i18n/AntdRootConfig';
import { DBInbound } from '@/models/dbinbound';
import { ThemeProvider } from '@/hooks/useTheme';
import { setDatepicker } from '@/hooks/useDatepicker';

import InboundFormModal from './InboundFormModal';

const meta = {
  title: 'Inbounds/InboundFormModal',
  component: InboundFormModal,
  tags: ['autodocs'],
  beforeEach: () => {
    setDatepicker('gregorian');
  },
  args: {
    open: false,
    mode: 'add',
    dbInbound: null,
    dbInbounds: [],
    onClose: () => {},
    onSaved: () => {},
  },
  parameters: {
    docs: {
      description: {
        component:
          'The real inbound form. Exercise named options, keyboard navigation, validation and modal focus here; verify TalkBack/Chrome and NVDA separately before claiming screen-reader compatibility.',
      },
    },
  },
} satisfies Meta<typeof InboundFormModal>;

export default meta;
type Story = StoryObj<typeof meta>;

function InboundDemo({ edit = false }: { edit?: boolean }) {
  const [open, setOpen] = useState(false);
  const [queryClient] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  const [inbound] = useState(
    () =>
      new DBInbound({
        id: 41,
        protocol: 'shadowsocks',
        enable: true,
        port: 14443,
        settings: {
          method: 'chacha20-ietf-poly1305',
          password: 'example-password',
          network: 'tcp,udp',
          clients: [],
        },
        streamSettings: { network: 'tcp', security: 'none', tcpSettings: {} },
      }),
  );
  return (
    <QueryClientProvider client={queryClient}>
      <AntdRootConfig>
        <ThemeProvider>
          <Button onClick={() => setOpen(true)}>{edit ? 'Edit inbound' : 'Add inbound'}</Button>
          <InboundFormModal
            open={open}
            mode={edit ? 'edit' : 'add'}
            dbInbound={edit ? inbound : null}
            dbInbounds={[]}
            availableNodes={[]}
            onClose={() => setOpen(false)}
            onSaved={() => {}}
          />
        </ThemeProvider>
      </AntdRootConfig>
    </QueryClientProvider>
  );
}

export const Create: Story = {
  render: () => <InboundDemo />,
};

export const SelectableOptions: Story = {
  ...Create,
  play: async ({ canvas, canvasElement, userEvent }) => {
    const page = within(canvasElement.ownerDocument.body);
    const opener = canvas.getByRole('button', { name: 'Add inbound' });
    await userEvent.click(opener);
    const dialog = await page.findByRole('dialog', { name: 'Add Inbound' });
    const protocol = within(dialog).getByRole('combobox', { name: 'Protocol' });
    await userEvent.selectOptions(
      protocol,
      within(protocol).getByRole('option', { name: 'shadowsocks' }),
    );
    await expect(protocol).toHaveDisplayValue('shadowsocks');
    await expect(protocol).toHaveFocus();
    await userEvent.click(within(dialog).getByRole('tab', { name: 'Protocol' }));
    const cipher = within(dialog).getByRole('combobox', { name: 'Encryption method' });
    await userEvent.selectOptions(
      cipher,
      within(cipher).getByRole('option', { name: 'chacha20-ietf-poly1305' }),
    );
    await userEvent.click(within(dialog).getByRole('tab', { name: 'Stream' }));
    const transport = within(dialog).getByRole('combobox', { name: 'Transmission' });
    await userEvent.selectOptions(
      transport,
      within(transport).getByRole('option', { name: 'WebSocket' }),
    );
    await expect(within(dialog).getByRole('textbox', { name: 'Path' })).toHaveValue('/');
    await userEvent.click(within(dialog).getAllByRole('button', { name: 'Close' })[0]);
    await waitFor(() => expect(opener).toHaveFocus());
  },
};

export const Edit: Story = {
  render: () => <InboundDemo edit />,
};
