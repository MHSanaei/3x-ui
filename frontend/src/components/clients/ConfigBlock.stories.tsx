import type { Meta, StoryObj } from '@storybook/react-vite';

import ConfigBlock from './ConfigBlock';

const meta = {
  title: 'Clients/ConfigBlock',
  component: ConfigBlock,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof ConfigBlock>;

export default meta;

type Story = StoryObj<typeof meta>;

const sampleLink = 'vless://11112222-3333-4444-5555-666677778888@panel.example.com:443'
  + '?type=ws&security=tls&path=%2Fpath#example-node';

export const Collapsed: Story = {
  args: { label: 'vless', text: sampleLink, fileName: 'client-config.txt' },
};

export const Expanded: Story = {
  args: { label: 'vless', text: sampleLink, fileName: 'client-config.txt', defaultOpen: true },
};

export const WithoutQr: Story = {
  args: { label: 'trojan', text: sampleLink, fileName: 'client-config.txt', showQr: false, tagColor: 'geekblue' },
};
