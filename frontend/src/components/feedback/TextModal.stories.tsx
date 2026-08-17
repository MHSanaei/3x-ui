import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, waitFor, within } from 'storybook/test';
import { Button } from 'antd';

import TextModal from './TextModal';

const meta = {
  title: 'Feedback/TextModal',
  component: TextModal,
  tags: ['autodocs'],
  parameters: {
    docs: {
      description: {
        component:
          'Read-only modal for viewing generated text or JSON — a client config, subscription, or exported settings — with copy and optional download actions, plus optional tabs for multiple documents.',
      },
    },
  },
  argTypes: {
    open: { description: 'Whether the modal is visible.' },
    title: { description: 'Modal title text.' },
    content: { description: 'Text shown when no `tabs` are provided.' },
    fileName: { description: 'When set, adds a download button that saves the active content under this name.' },
    json: { description: 'Render the content in a read-only JSON editor with syntax highlighting.' },
    tabs: { description: 'Optional list of `{ key, label, content }` documents shown as tabs.' },
    onClose: { description: 'Called when the modal is dismissed.' },
  },
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
  play: async ({ canvas, canvasElement, userEvent }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(canvas.getByRole('button', { name: 'Show config' }));
    const content = await body.findByDisplayValue(/vless:\/\/uuid@example\.com/);
    await waitFor(() => expect(content).toBeVisible());
    await expect(body.getByRole('button', { name: /client\.txt/ })).toBeEnabled();
  },
};

export const Json: Story = {
  args: placeholderArgs,
  render: () => <Demo json fileName="config.json" />,
};
