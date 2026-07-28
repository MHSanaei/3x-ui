import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { AllSetting } from '@/models/setting';
import { TelegramNotifications } from './TelegramNotifications';

const meta = {
  title: 'UI/Notifications/TelegramNotifications',
  component: TelegramNotifications,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Grid of event-group cards (outbound, Xray, node, system, security) that pick which panel events the Telegram bot reports, with per-group select-all and CPU/RAM threshold inputs. Used on the settings page Telegram tab to edit `tgEnabledEvents`.',
      },
    },
  },
  argTypes: {
    allSetting: { description: 'Panel settings snapshot; reads `tgEnabledEvents` plus the `tgCpu`/`tgMemory` thresholds.' },
    updateSetting: { description: 'Called with a partial settings patch when an event toggle or threshold changes.' },
  },
} satisfies Meta<typeof TelegramNotifications>;

export default meta;

type Story = StoryObj<typeof meta>;

function Demo({ initial }: { initial: AllSetting }) {
  const [settings, setSettings] = useState(initial);
  return (
    <TelegramNotifications
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
  render: () => <Demo initial={new AllSetting()} />,
};

export const TypicalMonitoring: Story = {
  args: placeholderArgs,
  render: () => (
    <Demo
      initial={
        new AllSetting({
          tgBotEnable: true,
          tgBotChatId: '123456789',
          tgEnabledEvents: 'xray.crash,node.down,cpu.high,memory.high,login.attempt',
          tgCpu: 85,
          tgMemory: 90,
        })
      }
    />
  ),
};

export const EverythingEnabled: Story = {
  args: placeholderArgs,
  render: () => (
    <Demo
      initial={
        new AllSetting({
          tgBotEnable: true,
          tgEnabledEvents:
            'outbound.down,outbound.up,xray.crash,node.down,node.up,cpu.high,memory.high,login.attempt',
          tgCpu: 70,
          tgMemory: 75,
        })
      }
    />
  ),
};
