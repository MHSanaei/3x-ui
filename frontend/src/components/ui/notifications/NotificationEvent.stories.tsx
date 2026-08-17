import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { InputNumber } from 'antd';

import { NotificationEvent } from './NotificationEvent';

const meta = {
  title: 'UI/Notifications/NotificationEvent',
  component: NotificationEvent,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Single toggleable notification event row used inside the Telegram and email notification groups on the settings page. Renders a checkbox with a translated label and, when checked, an optional indented extra control such as a threshold input.',
      },
    },
  },
  argTypes: {
    label: { description: 'i18n key (or already-translated text) shown next to the checkbox.' },
    checked: { description: 'Whether the event notification is enabled.' },
    onToggle: { description: 'Called when the checkbox is clicked.' },
    children: { description: 'Extra control rendered indented below the label while checked.' },
  },
} satisfies Meta<typeof NotificationEvent>;

export default meta;

type Story = StoryObj<typeof meta>;

function CpuThresholdDemo() {
  const [checked, setChecked] = useState(true);
  const [threshold, setThreshold] = useState(80);
  return (
    <NotificationEvent
      label="pages.settings.eventCPUHigh"
      checked={checked}
      onToggle={() => setChecked((prev) => !prev)}
    >
      <InputNumber
        size="small"
        min={0}
        max={100}
        value={threshold}
        onChange={(v) => setThreshold(v ?? 0)}
        aria-label="CPU usage threshold percent"
        style={{ width: 80 }}
      />
    </NotificationEvent>
  );
}

export const Unchecked: Story = {
  args: {
    label: 'pages.settings.eventLoginAttempt',
    checked: false,
    onToggle: () => undefined,
  },
};

export const Checked: Story = {
  args: {
    label: 'pages.settings.eventXrayCrash',
    checked: true,
    onToggle: () => undefined,
  },
};

export const CpuThreshold: Story = {
  args: {
    label: 'pages.settings.eventCPUHigh',
    checked: true,
    onToggle: () => undefined,
  },
  render: () => <CpuThresholdDemo />,
};
