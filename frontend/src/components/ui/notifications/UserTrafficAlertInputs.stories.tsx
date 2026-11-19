import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { AllSetting } from '@/models/setting';
import { UserTrafficAlertInputs } from './UserTrafficAlertInputs';

const meta = {
  title: 'UI/Notifications/UserTrafficAlertInputs',
  component: UserTrafficAlertInputs,
  tags: ['autodocs'],
  argTypes: {
    rule: { description: 'Selects sustained-rate or rolling-window settings.' },
    allSetting: { description: 'Saved mail notification settings with decimal MB/s and GB.' },
    updateSetting: { description: 'Receives a validated numeric field update.' },
  },
} satisfies Meta<typeof UserTrafficAlertInputs>;
export default meta;
type Story = StoryObj<typeof meta>;

function Editable({ rule }: { rule: 'rate' | 'window' }) {
  const [settings, setSettings] = useState(new AllSetting());
  return (
    <UserTrafficAlertInputs
      rule={rule}
      allSetting={settings}
      updateSetting={(patch) => setSettings((old) => new AllSetting({ ...old, ...patch }))}
    />
  );
}

export const Rate: Story = {
  args: { rule: 'rate', allSetting: new AllSetting(), updateSetting: () => undefined },
  render: () => <Editable rule="rate" />,
};
export const Window: Story = {
  args: { rule: 'window', allSetting: new AllSetting(), updateSetting: () => undefined },
  render: () => <Editable rule="window" />,
};
