import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { AllSetting } from '@/models/setting';
import { WechatNotifications } from './WechatNotifications';

const meta = {
  title: 'UI/Notifications/WechatNotifications',
  component: WechatNotifications,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Grid of grouped event checkboxes on the settings page that picks which panel events (outbound/node health, Xray crashes, CPU/RAM thresholds, login attempts) trigger a WeChat notification, stored as a comma-separated list in wechatEnabledEvents.',
      },
    },
  },
  argTypes: {
    allSetting: {
      description:
        'Panel settings snapshot; wechatEnabledEvents holds the selected event keys and wechatCpu/wechatMemory the alert threshold percentages.',
    },
    updateSetting: {
      description:
        'Receives a partial settings patch when an event is toggled or a threshold input changes.',
    },
  },
} satisfies Meta<typeof WechatNotifications>;

export default meta;

type Story = StoryObj<typeof meta>;

function StatefulDemo({ initial }: { initial: AllSetting }) {
  const [settings, setSettings] = useState(initial);
  return (
    <WechatNotifications
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
  render: () => <StatefulDemo initial={new AllSetting({ wechatEnabledEvents: '' })} />,
};

export const SystemThresholdAlerts: Story = {
  args: placeholderArgs,
  render: () => (
    <StatefulDemo
      initial={
        new AllSetting({
          wechatEnabledEvents: 'cpu.high,memory.high',
          wechatCpu: 85,
          wechatMemory: 90,
        })
      }
    />
  ),
};

export const InfrastructureOnly: Story = {
  args: placeholderArgs,
  render: () => (
    <StatefulDemo
      initial={
        new AllSetting({ wechatEnabledEvents: 'outbound.down,node.down,node.up,xray.crash' })
      }
    />
  ),
};

export const AllEventsEnabled: Story = {
  args: placeholderArgs,
  render: () => (
    <StatefulDemo
      initial={
        new AllSetting({
          wechatEnabledEvents:
            'outbound.down,outbound.up,xray.crash,node.down,node.up,cpu.high,memory.high,login.attempt',
          wechatCpu: 80,
          wechatMemory: 80,
        })
      }
    />
  ),
};
