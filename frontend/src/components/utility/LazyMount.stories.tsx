import { lazy, useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Alert, Button, Card, Layout, Modal, Skeleton, Space, Switch, Tag, Typography } from 'antd';
import { expect, userEvent, within } from 'storybook/test';

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
    when: {
      description: 'Children mount the first time this becomes true and stay mounted afterwards.',
    },
    fallback: {
      description:
        'Suspense fallback shown while a React.lazy child is loading. The default fullscreen spinner does not occupy space in the surrounding layout; custom fallbacks render as supplied.',
    },
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
        The card mounts the first time the switch turns on and stays mounted after turning it off;
        the mount time never changes.
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
          clients: [
            {
              id: 'b831381d-6324-4d53-ad4f-8cda48b30811',
              email: 'alice@corp.example',
              flow: 'xtls-rprx-vision',
            },
          ],
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

function LazyModalInLayoutDemo() {
  const [open, setOpen] = useState(false);
  const [SlowModal] = useState(() =>
    lazy(
      () =>
        new Promise<{ default: typeof Modal }>((resolve) => {
          setTimeout(() => resolve({ default: Modal }), 1200);
        }),
    ),
  );
  return (
    <Layout hasSider style={{ minHeight: 240 }}>
      <Layout.Sider width={96} style={{ color: '#fff' }}>
        Navigation
      </Layout.Sider>
      <Layout.Content data-testid="lazy-modal-content" style={{ padding: 16 }}>
        <Button onClick={() => setOpen(true)}>Open lazy dialog</Button>
        <Typography.Paragraph>
          The content width stays fixed while the dialog loads.
        </Typography.Paragraph>
      </Layout.Content>
      <LazyMount when={open}>
        <SlowModal title="Lazy dialog" open={open} onCancel={() => setOpen(false)} footer={null}>
          Dialog loaded without resizing the page.
        </SlowModal>
      </LazyMount>
    </Layout>
  );
}

export const LazyModalInLayout: Story = {
  args: placeholderArgs,
  render: () => <LazyModalInLayoutDemo />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const content = canvas.getByTestId('lazy-modal-content');
    const width = content.getBoundingClientRect().width;
    await userEvent.click(canvas.getByRole('button', { name: 'Open lazy dialog' }));
    const widths: number[] = [];
    for (let frame = 0; frame < 12; frame++) {
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
      widths.push(content.getBoundingClientRect().width);
    }
    expect(Math.min(...widths)).toBeCloseTo(width, 1);
    const page = within(canvasElement.ownerDocument.body);
    await page.findByRole('dialog', { name: 'Lazy dialog' }, { timeout: 5000 });
    await userEvent.click(page.getByRole('button', { name: 'Close' }));
  },
};
