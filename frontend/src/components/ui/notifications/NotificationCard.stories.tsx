import type { Meta, StoryObj } from '@storybook/react-vite';
import { Switch } from 'antd';
import { BellOutlined } from '@ant-design/icons';

import { NotificationCard } from './NotificationCard';

const meta = {
  title: 'UI/Notifications/NotificationCard',
  component: NotificationCard,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Small outlined card that groups a notification channel — an icon and title in the header, a control in the top-right `extra` slot (typically a toggle), and the channel settings as its body.',
      },
    },
  },
  argTypes: {
    icon: { description: 'Leading icon shown before the title.' },
    title: { description: 'Channel name shown in the header.' },
    extra: { description: 'Top-right slot, typically an enable/disable Switch.' },
    children: { description: 'Card body — the channel settings.' },
  },
} satisfies Meta<typeof NotificationCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    icon: <BellOutlined />,
    title: 'Telegram',
    extra: <Switch defaultChecked aria-label="Enable Telegram notifications" />,
    children: <span>Push a message to the configured chat when an event fires.</span>,
  },
};

export const Disabled: Story = {
  args: {
    icon: <BellOutlined />,
    title: 'Email',
    extra: <Switch aria-label="Enable email notifications" />,
    children: <span>Email delivery is turned off for this channel.</span>,
  },
};
