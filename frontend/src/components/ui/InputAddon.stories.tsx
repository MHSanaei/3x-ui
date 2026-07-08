import type { Meta, StoryObj } from '@storybook/react-vite';
import { Input, Space } from 'antd';

import InputAddon from './InputAddon';

const meta = {
  title: 'UI/InputAddon',
  component: InputAddon,
  tags: ['autodocs'],
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
      <Input defaultValue="panel.example.com" style={{ width: 220 }} />
    </Space.Compact>
  ),
};
