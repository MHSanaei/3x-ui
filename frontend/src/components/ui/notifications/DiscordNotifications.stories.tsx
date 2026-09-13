import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { AllSetting } from '@/models/setting';
import { DiscordNotifications } from './DiscordNotifications';

const meta = {
  title: 'UI/Notifications/DiscordNotifications',
  component: DiscordNotifications,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Grid of event-group cards (outbound, Xray, node, system, security) that pick which panel events the Discord bot reports, with per-group select-all and CPU/RAM threshold inputs. Used on the settings page Discord tab to edit `discordEnabledEvents`.',
      },
    },
  },
  argTypes: {
    allSetting: {
      description:
        'Panel settings snapshot; reads `discordEnabledEvents` plus the `discordCpu`/`discordMemory` thresholds.',
    },
    updateSetting: {
      description:
        'Called with a partial settings patch when an event toggle or threshold changes.',
    },
  },
} satisfies Meta<typeof DiscordNotifications>;

export default meta;

type Story = StoryObj<typeof meta>;

function Demo({ initial }: { initial: AllSetting }) {
  const [settings, setSettings] = useState(initial);
  return (
    <DiscordNotifications
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
          discordBotEnable: true,
          discordChannelId: '123456789012345678',
          discordEnabledEvents: 'xray.crash,node.down,cpu.high,memory.high,login.attempt',
          discordCpu: 85,
          discordMemory: 90,
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
          discordBotEnable: true,
          discordChannelId: '123456789012345678',
          discordEnabledEvents:
            'outbound.down,outbound.up,xray.crash,node.down,node.up,cpu.high,memory.high,login.attempt',
          discordCpu: 70,
          discordMemory: 75,
        })
      }
    />
  ),
};
