import type { Meta, StoryObj } from '@storybook/react-vite';

import { Status } from '@/models/status';
import ConnectionsCard from './ConnectionsCard';
import './IndexPage.css';

const samples = Array.from({ length: 72 }, (_, i) => Math.round(70 + (204 * i) / 71));

const meta = {
  title: 'Pages/Overview/ConnectionsCard',
  component: ConnectionsCard,
  parameters: { layout: 'padded' },
  decorators: [
    (Story) => (
      <div className="index-page ov-page" style={{ width: 900, maxWidth: '100%' }}>
        <Story />
      </div>
    ),
  ],
  args: {
    status: new Status({ tcpCount: 274, udpCount: 18 }),
    tcp: samples,
    udp: samples.map(() => 18),
    labels: samples.map((_, i) => `21:${String(i).padStart(2, '0')}`),
    isMobile: false,
  },
} satisfies Meta<typeof ConnectionsCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const TCPAndUDP: Story = {};
