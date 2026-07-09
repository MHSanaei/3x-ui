import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button } from 'antd';

import PromptModal from './PromptModal';

const meta = {
  title: 'Feedback/PromptModal',
  component: PromptModal,
  tags: ['autodocs'],
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
};

export const Textarea: Story = {
  args: placeholderArgs,
  render: () => <TextareaDemo />,
};
