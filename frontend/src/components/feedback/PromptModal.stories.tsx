import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, waitFor, within } from 'storybook/test';
import { Button } from 'antd';

import PromptModal from './PromptModal';

const meta = {
  title: 'Feedback/PromptModal',
  component: PromptModal,
  tags: ['autodocs'],
  parameters: {
    docs: {
      description: {
        component:
          'Modal that prompts for a single value — a plain input, a multi-line textarea, or a JSON editor. Confirms on Enter (input) or Ctrl+S (textarea) and returns the entered text.',
      },
    },
  },
  argTypes: {
    open: { description: 'Whether the modal is visible.' },
    title: { description: 'Modal title text.' },
    okText: { description: 'Confirm button label; defaults to the translated "confirm".' },
    type: { description: 'Editor variant: single-line `input` or multi-line `textarea`.' },
    initialValue: { description: 'Value pre-filled when the modal opens.' },
    loading: { description: 'Shows a loading state on the confirm button.' },
    json: { description: 'Render a JSON editor instead of a plain field.' },
    onConfirm: { description: 'Called with the entered value on confirm.' },
    onClose: { description: 'Called when the modal is dismissed.' },
  },
} satisfies Meta<typeof PromptModal>;

export default meta;

type Story = StoryObj<typeof meta>;

function InputDemo() {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState('');
  return (
    <>
      <Button type="primary" onClick={() => setOpen(true)}>Rename client</Button>
      <div style={{ marginTop: 12 }}>Last confirmed: {value || '—'}</div>
      <PromptModal
        open={open}
        title="Enter a new name"
        initialValue={value}
        onClose={() => setOpen(false)}
        onConfirm={(next) => {
          setValue(next);
          setOpen(false);
        }}
      />
    </>
  );
}

function TextareaDemo() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button onClick={() => setOpen(true)}>Edit note</Button>
      <PromptModal
        open={open}
        type="textarea"
        title="Edit note"
        initialValue={'line one\nline two'}
        onClose={() => setOpen(false)}
        onConfirm={() => setOpen(false)}
      />
    </>
  );
}

const placeholderArgs = {
  open: false,
  title: '',
  onClose: () => undefined,
  onConfirm: () => undefined,
};

export const Input: Story = {
  args: placeholderArgs,
  render: () => <InputDemo />,
  play: async ({ canvas, canvasElement, userEvent }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(canvas.getByRole('button', { name: 'Rename client' }));
    const input = await body.findByRole('textbox');
    await userEvent.type(input, 'new-name');
    await userEvent.keyboard('{Enter}');
    await expect(await canvas.findByText(/Last confirmed: new-name/)).toBeVisible();
  },
};

export const Textarea: Story = {
  args: placeholderArgs,
  render: () => <TextareaDemo />,
  play: async ({ canvas, canvasElement, userEvent }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(canvas.getByRole('button', { name: 'Edit note' }));
    const textarea = await body.findByRole('textbox');
    await expect(textarea).toHaveValue('line one\nline two');
    await userEvent.click(body.getByRole('button', { name: 'Confirm' }));
    await waitFor(() => expect(body.queryByRole('dialog')).not.toBeInTheDocument());
  },
};
