import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Checkbox } from 'antd';

import { NotificationHeader } from './NotificationHeader';

const meta = {
  title: 'UI/Notifications/NotificationHeader',
  component: NotificationHeader,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Selection summary for a notification group header — a `count/total` tag plus a tri-state master checkbox that selects or clears every event in the group. Rendered in the `extra` slot of the Telegram/email notification cards on the settings page.',
      },
    },
  },
  argTypes: {
    count: { description: 'Number of events currently selected in the group.' },
    total: { description: 'Total number of events the group offers.' },
    allSelected: { description: 'Checks the master checkbox when every event is selected.' },
    indeterminate: { description: 'Shows the dash state when only some events are selected.' },
    onToggleAll: { description: 'Called when the master checkbox is clicked to select or clear all events.' },
  },
} satisfies Meta<typeof NotificationHeader>;

export default meta;

type Story = StoryObj<typeof meta>;

export const NoneSelected: Story = {
  args: {
    count: 0,
    total: 6,
    allSelected: false,
    indeterminate: false,
    onToggleAll: () => undefined,
  },
};

export const PartialSelection: Story = {
  args: {
    count: 3,
    total: 6,
    allSelected: false,
    indeterminate: true,
    onToggleAll: () => undefined,
  },
};

export const AllSelected: Story = {
  args: {
    count: 6,
    total: 6,
    allSelected: true,
    indeterminate: false,
    onToggleAll: () => undefined,
  },
};

const events = ['Panel login', 'Xray crashed', 'CPU high', 'Client depleted'];

function GroupDemo() {
  const [selected, setSelected] = useState<string[]>(['Panel login', 'CPU high']);
  const count = selected.length;
  const total = events.length;
  function toggleAll() {
    setSelected(count === total ? [] : [...events]);
  }
  function toggle(name: string) {
    setSelected((prev) => (prev.includes(name) ? prev.filter((e) => e !== name) : [...prev, name]));
  }
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8, maxWidth: 260 }}>
      <NotificationHeader
        count={count}
        total={total}
        allSelected={count === total}
        indeterminate={count > 0 && count < total}
        onToggleAll={toggleAll}
      />
      {events.map((name) => (
        <Checkbox key={name} checked={selected.includes(name)} onChange={() => toggle(name)}>
          {name}
        </Checkbox>
      ))}
    </div>
  );
}

const placeholderArgs = {
  count: 0,
  total: 0,
  allSelected: false,
  indeterminate: false,
  onToggleAll: () => undefined,
};

export const Interactive: Story = {
  args: placeholderArgs,
  render: () => <GroupDemo />,
};
