import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { AllSetting } from '@/models/setting';
import WechatTab from './WechatTab';

const meta = {
  title: 'Pages/Settings/WechatTab',
  component: WechatTab,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof WechatTab>;

export default meta;
type Story = StoryObj<typeof meta>;

function SettingsPreview({ initial }: { initial: AllSetting }) {
  const [setting, setSetting] = useState(initial);
  return (
    <WechatTab
      allSetting={setting}
      hasPendingChanges={!setting.equals(initial)}
      updateSetting={(patch) => setSetting((current) => new AllSetting({ ...current, ...patch }))}
    />
  );
}

const placeholderArgs = { allSetting: new AllSetting(), updateSetting: () => undefined };

export const WecomRobot: Story = {
  args: placeholderArgs,
  render: () => <SettingsPreview initial={new AllSetting()} />,
};

export const SavedCustomGateway: Story = {
  args: placeholderArgs,
  render: () => (
    <SettingsPreview
      initial={
        new AllSetting({
          wechatEnable: true,
          wechatMode: 'webhook',
          hasWechatWebhookURL: true,
          hasWechatToken: true,
          wechatReportEnable: true,
        })
      }
    />
  ),
};

export const UnsavedChanges: Story = {
  args: {
    allSetting: new AllSetting({
      wechatEnable: true,
      wechatMode: 'webhook',
      wechatWebhookURL: 'https://receiver.example/notify',
    }),
    updateSetting: () => undefined,
    hasPendingChanges: true,
  },
};
