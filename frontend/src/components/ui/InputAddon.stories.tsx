import type { Meta, StoryObj } from '@storybook/react-vite';
import { Input, Space } from 'antd';

import InputAddon from './InputAddon';

const meta = {
  title: 'UI/InputAddon',
  component: InputAddon,
  tags: ['autodocs'],
  parameters: {
    docs: {
      description: {
        component:
          'Prefix/suffix addon styled to sit flush against an Ant Design input. Becomes a keyboard-accessible button (role, tabIndex, Enter/Space) when `onClick` is provided.',
      },
    },
  },
  argTypes: {
    children: { description: 'Addon content (text or an icon).' },
    onClick: { description: 'When set, the addon becomes an activatable button.' },
    ariaLabel: { description: 'Accessible label; used only when `onClick` is set.' },
    className: { description: 'Extra CSS class appended to the addon.' },
    style: { description: 'Inline styles for the addon element.' },
  },
} satisfies Meta<typeof InputAddon>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Static: Story = {
  args: { children: 'https://' },
};

export const Clickable: Story = {
  args: { children: 'Copy', ariaLabel: 'Copy value', onClick: () => undefined },
};

export const BesideInput: Story = {
  args: { children: 'https://' },
  render: () => (
    <Space.Compact>
      <InputAddon>https://</InputAddon>
      <Input defaultValue="panel.example.com" aria-label="Panel host" style={{ width: 220 }} />
    </Space.Compact>
  ),
};
