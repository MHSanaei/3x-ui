import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button, Input, Popover, Typography } from 'antd';

import { previewRemark, wrapToken } from '@/lib/remark/remarkVariables';

import RemarkVarPicker from './RemarkVarPicker';

const meta = {
  title: 'Form/RemarkVarPicker',
  component: RemarkVarPicker,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Grouped, tooltipped chip list of the {{VAR}} tokens the backend substitutes per client in subscription remarks. The hosts page shows it in a popover beside the remark-template field so operators can insert placeholders like {{EMAIL}} or {{TRAFFIC_LEFT}}.',
      },
    },
  },
  argTypes: {
    onPick: { description: 'Called with the bare token (e.g. "EMAIL") when a chip is clicked or activated via keyboard.' },
  },
} satisfies Meta<typeof RemarkVarPicker>;

export default meta;

type Story = StoryObj<typeof meta>;

function TemplateBuilderDemo() {
  const [template, setTemplate] = useState('{{INBOUND}}-{{EMAIL}} {{STATUS_EMOJI}} {{TRAFFIC_LEFT}} left');
  return (
    <div style={{ maxWidth: 520 }}>
      <Input
        value={template}
        onChange={(e) => setTemplate(e.target.value)}
        aria-label="Remark template"
        style={{ fontFamily: 'monospace' }}
      />
      <Typography.Paragraph type="secondary" style={{ margin: '8px 0 16px' }}>
        Preview: {previewRemark(template)}
      </Typography.Paragraph>
      <RemarkVarPicker onPick={(token) => setTemplate((prev) => `${prev}${wrapToken(token)}`)} />
    </div>
  );
}

function PopoverDemo() {
  const [lastPicked, setLastPicked] = useState('');
  return (
    <>
      <Popover
        content={<RemarkVarPicker onPick={(token) => setLastPicked(wrapToken(token))} />}
        trigger="click"
        placement="bottomRight"
        title="Remark variables"
      >
        <Button>Insert variable</Button>
      </Popover>
      <div style={{ marginTop: 12 }}>Last picked: {lastPicked || '—'}</div>
    </>
  );
}

export const Default: Story = {
  args: { onPick: () => undefined },
};

export const TemplateBuilder: Story = {
  args: { onPick: () => undefined },
  render: () => <TemplateBuilderDemo />,
};

export const InsidePopover: Story = {
  args: { onPick: () => undefined },
  render: () => <PopoverDemo />,
};
