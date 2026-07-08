import type { Meta, StoryObj } from '@storybook/react-vite';
import { Switch } from 'antd';
import { BellOutlined } from '@ant-design/icons';

import { NotificationCard } from './NotificationCard';

const meta = {
  title: 'UI/Notifications/NotificationCard',
  component: NotificationCard,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof NotificationCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    icon: <BellOutlined />,
    title: 'Telegram',
    extra: <Switch defaultChecked />,
    children: <span>Push a message to the configured chat when an event fires.</span>,
  },
};

export const Disabled: Story = {
  args: {
    icon: <BellOutlined />,
    title: 'Email',
    extra: <Switch />,
    children: <span>Email delivery is turned off for this channel.</span>,
  },
};
