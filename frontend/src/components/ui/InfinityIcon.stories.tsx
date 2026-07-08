import type { Meta, StoryObj } from '@storybook/react-vite';

import InfinityIcon from './InfinityIcon';

const meta = {
  title: 'UI/InfinityIcon',
  component: InfinityIcon,
  tags: ['autodocs'],
} satisfies Meta<typeof InfinityIcon>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Large: Story = {
  args: { width: 48, height: 34 },
};

export const InlineWithText: Story = {
  render: () => (
    <span style={{ fontSize: 16 }}>
      Unlimited traffic <InfinityIcon />
    </span>
  ),
};
