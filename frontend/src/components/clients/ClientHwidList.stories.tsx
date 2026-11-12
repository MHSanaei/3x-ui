import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';

import ClientHwidListModal from './ClientHwidList';

const meta = {
  title: 'Clients/HWID Devices',
  component: ClientHwidListModal,
  args: {
    open: true,
    email: 'subscriber@example.com',
    loading: false,
    clearing: false,
    deletingId: null,
    blockingId: null,
    formatDate: (ts: number) => new Date(ts).toISOString().replace('T', ' ').slice(0, 16),
    onRefresh: fn(),
    onClearAll: fn(),
    onDelete: fn(),
    onSetBlocked: fn(),
    onClose: fn(),
  },
} satisfies Meta<typeof ClientHwidListModal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const AllowedAndBlocked: Story = {
  args: {
    hwids: [
      {
        id: 1,
        blocked: false,
        firstSeen: 1794225600000,
        lastSeen: 1794484800000,
        userAgent: 'Happ/4.0',
        deviceOs: 'Android',
        osVersion: '15',
        deviceModel: 'Pixel 9',
        fingerprint: '6ad17c93e821',
      },
      {
        id: 2,
        blocked: true,
        firstSeen: 1794225600000,
        lastSeen: 1794398400000,
        userAgent: 'Happ/4.0',
        deviceOs: 'iOS',
        osVersion: '18',
        deviceModel: 'iPhone',
        fingerprint: '9d241cb7e506',
      },
    ],
  },
};
