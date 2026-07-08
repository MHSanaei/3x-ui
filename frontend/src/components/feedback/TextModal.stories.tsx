import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button } from 'antd';

import TextModal from './TextModal';

const meta = {
  title: 'Feedback/TextModal',
  component: TextModal,
  tags: ['autodocs'],
} satisfies Meta<typeof TextModal>;

export default meta;

type Story = StoryObj<typeof meta>;

const jsonSample = JSON.stringify({ outbounds: [{ protocol: 'vless', tag: 'proxy' }] }, null, 2);

function Demo({ json, fileName }: { json?: boolean; fileName?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button onClick={() => setOpen(true)}>Show config</Button>
      <TextModal
        open={open}
        title="Client configuration"
        content={json ? jsonSample : 'vless://uuid@example.com:443#node'}
        fileName={fileName}
        json={json}
        onClose={() => setOpen(false)}
      />
    </>
  );
}

const placeholderArgs = {
  open: false,
  title: '',
  content: '',
  onClose: () => undefined,
};

export const PlainText: Story = {
  args: placeholderArgs,
  render: () => <Demo fileName="client.txt" />,
};

export const Json: Story = {
  args: placeholderArgs,
  render: () => <Demo json fileName="config.json" />,
};
