import { lazy, useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Alert, Button, Card, Skeleton, Space, Switch, Tag, Typography } from 'antd';

import LazyMount from './LazyMount';

const meta = {
  title: 'Utility/LazyMount',
  component: LazyMount,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Mounts its children the first time `when` becomes true and keeps them mounted afterwards, wrapped in Suspense. The panel pairs it with React.lazy modal imports on heavy list pages so modals load on demand while their close animations still play.',
      },
    },
  },
  argTypes: {
    when: { description: 'Children mount the first time this becomes true and stay mounted afterwards.' },
    fallback: { description: 'Suspense fallback shown while a React.lazy child is still loading.' },
    children: { description: 'Content to mount on demand, typically a lazily imported modal.' },
  },
} satisfies Meta<typeof LazyMount>;

export default meta;

type Story = StoryObj<typeof meta>;

function MountBadge() {
  const [mountedAt] = useState(() => new Date().toLocaleTimeString());
  return <Tag color="green">mounted at {mountedAt}</Tag>;
}

function OnDemandDemo() {
  const [visible, setVisible] = useState(false);
  return (
    <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
      <Space>
        <Switch checked={visible} onChange={setVisible} aria-label="Mount the client card" />
        <Typography.Text>when = {String(visible)}</Typography.Text>
      </Space>
      <LazyMount when={visible}>
        <Card size="small" title="alice@corp.example" style={{ maxWidth: 480 }}>
          <Space orientation="vertical" size={4}>
            <MountBadge />
            <Typography.Text type="secondary">Traffic: 42.7 GB of 100 GB</Typography.Text>
            <Typography.Text code copyable style={{ fontSize: 12 }}>
              vless://b831381d-6324-4d53-ad4f-8cda48b30811@vpn.example.com:443?type=tcp&security=reality&fp=chrome&sni=yahoo.com#alice
            </Typography.Text>
          </Space>
        </Card>
      </LazyMount>
      <Typography.Text type="secondary">
        The card mounts the first time the switch turns on and stays mounted after turning it off; the mount time never changes.
      </Typography.Text>
    </Space>
  );
}

const xrayConfigSnippet = JSON.stringify(
  {
    inbounds: [
      {
        tag: 'inbound-443',
        protocol: 'vless',
        port: 443,
        settings: {
          clients: [{ id: 'b831381d-6324-4d53-ad4f-8cda48b30811', email: 'alice@corp.example', flow: 'xtls-rprx-vision' }],
          decryption: 'none',
        },
        streamSettings: { network: 'tcp', security: 'reality' },
      },
    ],
  },
  null,
  2,
);

function XrayConfigPreview() {
  return (
    <Card size="small" title="Generated xray config">
      <pre style={{ margin: 0, overflowX: 'auto', fontSize: 12 }}>{xrayConfigSnippet}</pre>
    </Card>
  );
}

const SlowXrayConfigPreview = lazy(
  () =>
    new Promise<{ default: typeof XrayConfigPreview }>((resolve) => {
      setTimeout(() => resolve({ default: XrayConfigPreview }), 1200);
    }),
);

function LazyChildDemo() {
  const [open, setOpen] = useState(false);
  return (
    <Space orientation="vertical" size="middle" style={{ width: '100%', maxWidth: 560 }}>
      <Button type="primary" onClick={() => setOpen(true)} disabled={open}>
        Load config preview
      </Button>
      <LazyMount when={open} fallback={<Skeleton active paragraph={{ rows: 4 }} />}>
        <SlowXrayConfigPreview />
      </LazyMount>
    </Space>
  );
}

const placeholderArgs = {
  when: false,
  children: null,
};

export const MountOnDemand: Story = {
  args: placeholderArgs,
  render: () => <OnDemandDemo />,
};

export const LazyChildWithFallback: Story = {
  args: placeholderArgs,
  render: () => <LazyChildDemo />,
};

export const MountedImmediately: Story = {
  args: {
    when: true,
    children: (
      <Alert
        type="warning"
        showIcon
        title="Client depleted"
        description="bob@corp.example used 100 GB of 100 GB and was disabled by the traffic job."
        style={{ maxWidth: 480 }}
      />
    ),
  },
};
