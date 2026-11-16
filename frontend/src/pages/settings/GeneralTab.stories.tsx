import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Card } from 'antd';
import { expect, within } from 'storybook/test';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { AllSetting } from '@/models/setting';
import { HttpUtil, Msg } from '@/utils';
import GeneralTab from './GeneralTab';

const meta = {
  title: 'Settings/General',
  component: GeneralTab,
  parameters: { layout: 'padded' },
  beforeEach: () => {
    const get = HttpUtil.get;
    const post = HttpUtil.post;
    HttpUtil.get = async <T,>() => new Msg<T>(true);
    HttpUtil.post = async <T,>() => new Msg<T>(true);
    return () => {
      HttpUtil.get = get;
      HttpUtil.post = post;
    };
  },
} satisfies Meta<typeof GeneralTab>;

export default meta;
type Story = StoryObj<typeof meta>;

function GeneralSettingsDemo() {
  const [settings, setSettings] = useState(() => new AllSetting({ tcpPerIpConnectionLimit: 200 }));
  const [queryClient] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={queryClient}>
      <Card title="General settings" style={{ maxWidth: 1200 }}>
        <GeneralTab
          allSetting={settings}
          updateSetting={(patch) => setSettings((prev) => new AllSetting({ ...prev, ...patch }))}
        />
      </Card>
    </QueryClientProvider>
  );
}

export const TCPConnectionProtection: Story = {
  args: { allSetting: new AllSetting(), updateSetting: () => undefined },
  render: () => <GeneralSettingsDemo />,
  play: async ({ canvasElement }) => {
    const input = within(canvasElement).getByRole('spinbutton', {
      name: 'TCP connections per IP',
    });
    await expect(input).toHaveValue('200');
    input.scrollIntoView({ block: 'center' });
  },
};
