import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { AllSetting } from '@/models/setting';
import { EmailNotifications } from './EmailNotifications';

const meta = {
  title: 'UI/Notifications/EmailNotifications',
  component: EmailNotifications,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Grid of grouped event checkboxes on the settings page that picks which panel events (outbound/node health, Xray crashes, CPU/RAM thresholds, login attempts) trigger an SMTP email, stored as a comma-separated list in smtpEnabledEvents.',
      },
    },
  },
  argTypes: {
    allSetting: {
      description:
        'Panel settings snapshot; smtpEnabledEvents holds the selected event keys and smtpCpu/smtpMemory the alert threshold percentages.',
    },
    updateSetting: {
      description: 'Receives a partial settings patch when an event is toggled or a threshold input changes.',
    },
  },
} satisfies Meta<typeof EmailNotifications>;

export default meta;

type Story = StoryObj<typeof meta>;

function StatefulDemo({ initial }: { initial: AllSetting }) {
  const [settings, setSettings] = useState(initial);
  return (
    <EmailNotifications
      allSetting={settings}
      updateSetting={(patch) => setSettings((prev) => new AllSetting({ ...prev, ...patch }))}
    />
  );
}

const placeholderArgs = {
  allSetting: new AllSetting(),
  updateSetting: () => undefined,
};

export const NothingSelected: Story = {
  args: placeholderArgs,
  render: () => <StatefulDemo initial={new AllSetting()} />,
};

export const SystemThresholdAlerts: Story = {
  args: placeholderArgs,
  render: () => (
    <StatefulDemo
      initial={new AllSetting({ smtpEnabledEvents: 'cpu.high,memory.high', smtpCpu: 85, smtpMemory: 90 })}
    />
  ),
};

export const InfrastructureOnly: Story = {
  args: placeholderArgs,
  render: () => (
    <StatefulDemo initial={new AllSetting({ smtpEnabledEvents: 'outbound.down,node.down,node.up,xray.crash' })} />
  ),
};

export const AllEventsEnabled: Story = {
  args: placeholderArgs,
  render: () => (
    <StatefulDemo
      initial={
        new AllSetting({
          smtpEnabledEvents:
            'outbound.down,outbound.up,xray.crash,node.down,node.up,cpu.high,memory.high,login.attempt',
          smtpCpu: 80,
          smtpMemory: 80,
        })
      }
    />
  ),
};
