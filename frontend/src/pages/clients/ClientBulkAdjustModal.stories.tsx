import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, within } from 'storybook/test';

import { ThemeProvider } from '@/hooks/useTheme';
import { setDatepicker } from '@/hooks/useDatepicker';

import ClientBulkAdjustModal from './ClientBulkAdjustModal';
import './ClientsPage.css';

const meta = {
  title: 'Clients/Bulk Adjust',
  component: ClientBulkAdjustModal,
  tags: ['autodocs'],
  decorators: [
    (Story) => (
      <ThemeProvider>
        <Story />
      </ThemeProvider>
    ),
  ],
  beforeEach: () => {
    setDatepicker('gregorian');
  },
  args: {
    open: true,
    count: 3,
    onOpenChange: fn(),
    onSubmit: fn().mockResolvedValue({
      adjusted: 3,
      externalLinks: { added: 6, skipped: 0, errors: 0 },
    }),
  },
  argTypes: {
    count: { description: 'Number of selected clients receiving the active adjustment.' },
    onSubmit: {
      description: 'Applies either settings adjustments or an append-only external-link batch.',
    },
  },
} satisfies Meta<typeof ClientBulkAdjustModal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const AppendExternalLinks: Story = {
  play: async ({ canvasElement, userEvent }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(body.getByRole('tab', { name: 'Append external links' }));
    await userEvent.click(body.getByRole('button', { name: 'Add External Link' }));
    await userEvent.type(
      body.getByPlaceholderText(
        'vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://',
      ),
      'vless://uuid@backup.example:443',
    );
    await userEvent.click(body.getByRole('button', { name: 'Add External Subscription' }));
    await userEvent.type(
      body.getByPlaceholderText('https://provider.example/sub/…'),
      'https://provider.example/sub',
    );
    await userEvent.type(body.getByLabelText('Name prefix'), '[[backup] ');
    await expect(body.getByLabelText('Name prefix')).toHaveValue('[backup] ');
    await expect(body.getByRole('dialog', { name: 'Adjust 3 clients' })).toBeVisible();
  },
};
