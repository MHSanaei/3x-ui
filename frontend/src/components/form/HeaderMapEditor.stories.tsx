import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import HeaderMapEditor, { type HeaderMapValue } from './HeaderMapEditor';

const meta = {
  title: 'Form/HeaderMapEditor',
  component: HeaderMapEditor,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Row-based editor for Xray HTTP header maps, used in the inbound/outbound stream forms. Mode `v1` emits one string per header name (WS / HTTPUpgrade / Hysteria masquerade); mode `v2` emits string arrays so headers can repeat (TCP HTTP camouflage request/response).',
      },
    },
  },
  argTypes: {
    mode: { description: 'Wire shape: `v1` = string per name, `v2` = string[] per name (repeatable headers).' },
    value: { description: 'Header map in the wire shape matching `mode`; converted to editable rows internally.' },
    onChange: { description: 'Called with the rebuilt wire-shape map after every row edit, add, or remove.' },
  },
} satisfies Meta<typeof HeaderMapEditor>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: { mode: 'v1', onChange: () => undefined },
};

export const WsHostHeaders: Story = {
  args: {
    mode: 'v1',
    value: {
      Host: 'cdn.example.com',
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
    },
    onChange: () => undefined,
  },
};

export const TcpCamouflageRequest: Story = {
  args: {
    mode: 'v2',
    value: {
      Accept: ['text/html,application/xhtml+xml', 'application/json'],
      'Accept-Encoding': ['gzip, deflate'],
      Connection: ['keep-alive'],
      Pragma: ['no-cache'],
    },
    onChange: () => undefined,
  },
};

function WireShapeDemo() {
  const [value, setValue] = useState<HeaderMapValue>({
    Accept: ['text/html', 'application/json'],
    'X-Forwarded-For': ['203.0.113.7'],
  });
  return (
    <div style={{ maxWidth: 560 }}>
      <HeaderMapEditor mode="v2" value={value} onChange={setValue} />
      <pre style={{ marginTop: 16, padding: 12, borderRadius: 8, background: 'rgba(128, 128, 128, 0.12)' }}>
        {JSON.stringify(value ?? {}, null, 2)}
      </pre>
    </div>
  );
}

export const LiveWireShape: Story = {
  args: { mode: 'v2', onChange: () => undefined },
  render: () => <WireShapeDemo />,
};
