import type { Meta, StoryObj } from '@storybook/react-vite';
import { InputNumber, Space } from 'antd';
import {
  CloudServerOutlined,
  DashboardOutlined,
  DesktopOutlined,
  SafetyOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import { NotificationLayout } from './NotificationLayout';
import { NotificationCard } from './NotificationCard';
import { NotificationEvent } from './NotificationEvent';
import { NotificationHeader } from './NotificationHeader';

const noop = () => undefined;

function OutboundGroup() {
  return (
    <NotificationCard
      icon={<CloudServerOutlined />}
      title="Outbound"
      extra={<NotificationHeader count={1} total={2} allSelected={false} indeterminate onToggleAll={noop} />}
    >
      <Space orientation="vertical" size={8} style={{ width: '100%' }}>
        <NotificationEvent label="Outbound went down" checked onToggle={noop} />
        <NotificationEvent label="Outbound recovered" checked={false} onToggle={noop} />
      </Space>
    </NotificationCard>
  );
}

function XrayGroup() {
  return (
    <NotificationCard
      icon={<ThunderboltOutlined />}
      title="Xray"
      extra={<NotificationHeader count={1} total={1} allSelected indeterminate={false} onToggleAll={noop} />}
    >
      <Space orientation="vertical" size={8} style={{ width: '100%' }}>
        <NotificationEvent label="Xray crashed" checked onToggle={noop} />
      </Space>
    </NotificationCard>
  );
}

function NodeGroup() {
  return (
    <NotificationCard
      icon={<DesktopOutlined />}
      title="Nodes"
      extra={<NotificationHeader count={0} total={2} allSelected={false} indeterminate={false} onToggleAll={noop} />}
    >
      <Space orientation="vertical" size={8} style={{ width: '100%' }}>
        <NotificationEvent label="Node went offline" checked={false} onToggle={noop} />
        <NotificationEvent label="Node back online" checked={false} onToggle={noop} />
      </Space>
    </NotificationCard>
  );
}

function SystemGroup() {
  return (
    <NotificationCard
      icon={<DashboardOutlined />}
      title="System"
      extra={<NotificationHeader count={2} total={2} allSelected indeterminate={false} onToggleAll={noop} />}
    >
      <Space orientation="vertical" size={8} style={{ width: '100%' }}>
        <NotificationEvent label="CPU usage above threshold (%)" checked onToggle={noop}>
          <InputNumber size="small" min={0} max={100} defaultValue={80} aria-label="CPU usage threshold percent" style={{ width: 80 }} />
        </NotificationEvent>
        <NotificationEvent label="Memory usage above threshold (%)" checked onToggle={noop}>
          <InputNumber size="small" min={0} max={100} defaultValue={90} aria-label="Memory usage threshold percent" style={{ width: 80 }} />
        </NotificationEvent>
      </Space>
    </NotificationCard>
  );
}

function SecurityGroup() {
  return (
    <NotificationCard
      icon={<SafetyOutlined />}
      title="Security"
      extra={<NotificationHeader count={1} total={1} allSelected indeterminate={false} onToggleAll={noop} />}
    >
      <Space orientation="vertical" size={8} style={{ width: '100%' }}>
        <NotificationEvent label="Panel login attempt" checked onToggle={noop} />
      </Space>
    </NotificationCard>
  );
}

const meta = {
  title: 'UI/Notifications/NotificationLayout',
  component: NotificationLayout,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Responsive auto-fit grid (min 260px columns) that arranges notification event-group cards; the Telegram and email notification tabs on the settings page render their groups inside it.',
      },
    },
  },
  argTypes: {
    children: { description: 'Grid items, typically one NotificationCard per event group.' },
  },
} satisfies Meta<typeof NotificationLayout>;

export default meta;

type Story = StoryObj<typeof meta>;

export const AllEventGroups: Story = {
  args: {
    children: (
      <>
        <OutboundGroup />
        <XrayGroup />
        <NodeGroup />
        <SystemGroup />
        <SecurityGroup />
      </>
    ),
  },
};

export const TwoGroups: Story = {
  args: {
    children: (
      <>
        <SystemGroup />
        <SecurityGroup />
      </>
    ),
  },
};

export const SingleGroup: Story = {
  args: {
    children: <OutboundGroup />,
  },
};
