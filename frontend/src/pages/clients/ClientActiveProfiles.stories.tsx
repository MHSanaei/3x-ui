import type { Meta, StoryObj } from '@storybook/react-vite';
import { Table } from 'antd';

import { ClientActiveProfiles } from './RowCells';

const meta = {
  title: 'Clients/ClientActiveProfiles',
  component: ClientActiveProfiles,
  tags: ['autodocs'],
  args: {
    email: 'alice',
    ids: [1, 2],
    inboundsById: {
      1: {
        id: 1,
        tag: 'in-vless',
        remark: 'VLESS — node A',
        protocol: 'vless',
        originNodeGuid: 'node-a',
      },
      2: {
        id: 2,
        tag: 'in-hysteria',
        remark: 'Hysteria2 — node B',
        protocol: 'hysteria',
        originNodeGuid: 'node-b',
      },
      3: {
        id: 3,
        tag: 'legacy',
        remark: 'Older node',
        protocol: 'vless',
        originNodeGuid: 'node-c',
      },
    },
    protocolColors: { vless: 'blue', hysteria: 'purple' },
    tree: {
      'node-a': {
        supported: true,
        observedAt: 1,
        clients: { alice: ['in-vless'], bob: ['in-vless'] },
      },
      'node-b': { supported: true, observedAt: 1, clients: { alice: ['in-hysteria'] } },
      'node-c': { supported: false, observedAt: 1, clients: {} },
    },
  },
  parameters: {
    docs: {
      description: {
        component:
          'Live authenticated profiles, scoped by client email, physical node and inbound tag. These fixtures are synthetic; no production clients or credentials are used.',
      },
    },
  },
} satisfies Meta<typeof ClientActiveProfiles>;

export default meta;
type Story = StoryObj<typeof meta>;

export const ConcurrentAcrossNodes: Story = {};
export const Disconnected: Story = { args: { email: 'carol' } };
export const Unknown: Story = { args: { ids: [3] } };

export const Overview: Story = {
  render: (args) => (
    <Table
      pagination={false}
      rowKey="email"
      style={{ maxWidth: 900 }}
      dataSource={[
        { email: 'alice', label: 'Alice · shared account', ids: [1, 2] },
        { email: 'bob', label: 'Bob', ids: [1, 2] },
        { email: 'carol', label: 'Carol · no open links', ids: [1, 2] },
        { email: 'dave', label: 'Dave · older core', ids: [3] },
      ]}
      columns={[
        { title: 'Client', dataIndex: 'label', width: 260 },
        {
          title: 'Active profiles',
          render: (_, row) => <ClientActiveProfiles {...args} email={row.email} ids={row.ids} />,
        },
      ]}
    />
  ),
};
