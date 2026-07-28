import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { InputNumber } from 'antd';
import { CloudServerOutlined, DashboardOutlined } from '@ant-design/icons';

import { AllSetting } from '@/models/setting';
import { NotificationGroup } from './NotificationGroup';
import type { NotificationGroupConfig } from './types';

const systemGroup: NotificationGroupConfig = {
  icon: <DashboardOutlined />,
  title: 'eventGroupSystem',
  events: [
    {
      key: 'cpu.high',
      label: 'eventCPUHigh',
      settingKey: 'tgCpu',
      extra: ({ value, onChange, ariaLabel }) => (
        <InputNumber size="small" min={0} max={100} value={value} onChange={onChange} aria-label={ariaLabel} style={{ width: 80 }} />
      ),
    },
    {
      key: 'memory.high',
      label: 'eventMemoryHigh',
      settingKey: 'tgMemory',
      extra: ({ value, onChange, ariaLabel }) => (
        <InputNumber size="small" min={0} max={100} value={value} onChange={onChange} aria-label={ariaLabel} style={{ width: 80 }} />
      ),
    },
  ],
};

const outboundGroup: NotificationGroupConfig = {
  icon: <CloudServerOutlined />,
  title: 'eventGroupOutbound',
  events: [
    { key: 'outbound.down', label: 'eventOutboundDown', settingKey: '' },
    { key: 'outbound.up', label: 'eventOutboundUp', settingKey: '' },
  ],
};

const meta = {
  title: 'UI/Notifications/NotificationGroup',
  component: NotificationGroup,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Card for one notification event group (outbound, Xray, node, system, security) with a per-group select-all checkbox, a selected-count tag, and optional per-event threshold inputs. Composed by the Telegram and email notification tabs on the settings page.',
      },
    },
  },
  argTypes: {
    config: { description: 'Group definition: icon, `pages.settings` title key, and the event rows to render.' },
    selected: { description: 'Enabled event keys; drives each checkbox and the header count.' },
    onToggle: { description: 'Called with the event key when a single checkbox is clicked.' },
    onToggleAll: { description: 'Called with every event key in the group when the master checkbox is clicked.' },
    allSetting: { description: 'Panel settings snapshot; threshold values such as `tgCpu` are read from it.' },
    updateSetting: { description: 'Called with a partial settings patch when a threshold input changes.' },
  },
} satisfies Meta<typeof NotificationGroup>;

export default meta;

type Story = StoryObj<typeof meta>;

function Demo() {
  const [selected, setSelected] = useState<string[]>(['cpu.high']);
  const [settings, setSettings] = useState(new AllSetting({ tgCpu: 85, tgMemory: 90 }));
  return (
    <NotificationGroup
      config={systemGroup}
      selected={selected}
      onToggle={(key) =>
        setSelected((prev) => (prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]))
      }
      onToggleAll={(keys) =>
        setSelected((prev) => (keys.every((k) => prev.includes(k)) ? prev.filter((k) => !keys.includes(k)) : [...new Set([...prev, ...keys])]))
      }
      allSetting={settings}
      updateSetting={(patch) => setSettings((prev) => new AllSetting({ ...prev, ...patch }))}
    />
  );
}

export const AllSelected: Story = {
  args: {
    config: systemGroup,
    selected: ['cpu.high', 'memory.high'],
    onToggle: () => undefined,
    onToggleAll: () => undefined,
    allSetting: new AllSetting({ tgCpu: 85, tgMemory: 90 }),
    updateSetting: () => undefined,
  },
};

export const PartiallySelected: Story = {
  args: {
    config: systemGroup,
    selected: ['cpu.high'],
    onToggle: () => undefined,
    onToggleAll: () => undefined,
    allSetting: new AllSetting(),
    updateSetting: () => undefined,
  },
};

export const NoneSelected: Story = {
  args: {
    config: outboundGroup,
    selected: [],
    onToggle: () => undefined,
    onToggleAll: () => undefined,
    allSetting: new AllSetting(),
    updateSetting: () => undefined,
  },
};

export const Interactive: Story = {
  args: {
    config: systemGroup,
    selected: [],
    onToggle: () => undefined,
    onToggleAll: () => undefined,
    allSetting: new AllSetting(),
    updateSetting: () => undefined,
  },
  render: () => <Demo />,
};
