import { useEffect, useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Typography } from 'antd';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';

import { setDatepicker } from '@/hooks/useDatepicker';
import { ThemeProvider } from '@/hooks/useTheme';

import DateTimePicker from './DateTimePicker';

setDatepicker('gregorian');

function ClientExpiryDemo() {
  const [value, setValue] = useState<Dayjs | null>(dayjs('2026-12-31 23:59:59'));
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <DateTimePicker value={value} onChange={setValue} placeholder="Expiry date" />
      <Typography.Text type="secondary">
        {value ? `user1@node-de expiryTime: ${value.valueOf()}` : 'user1@node-de expiryTime: 0 (never expires)'}
      </Typography.Text>
    </div>
  );
}

function JalaliDemo() {
  const [value, setValue] = useState<Dayjs | null>(dayjs('2026-12-31 23:59:59'));
  useEffect(() => {
    setDatepicker('jalalian');
    return () => setDatepicker('gregorian');
  }, []);
  return <DateTimePicker value={value} onChange={setValue} placeholder="Expiry date" />;
}

const meta = {
  title: 'Form/DateTimePicker',
  component: DateTimePicker,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Calendar-aware date/time picker used for client and inbound expiry dates. Renders an AntD DatePicker by default and switches to a Persian (Jalali) calendar — with theme-matched colors and an overlaid clear button — when the panel datepicker setting is jalalian.',
      },
    },
  },
  decorators: [
    (Story) => (
      <ThemeProvider>
        <Story />
      </ThemeProvider>
    ),
  ],
  argTypes: {
    value: { description: 'Selected moment as a Dayjs instance, or null when unset.' },
    onChange: { description: 'Called with the picked Dayjs value, or null when cleared.' },
    showTime: { description: 'Include an hour/minute/second selector alongside the date.' },
    format: { description: 'Display format for the Gregorian picker input.' },
    placeholder: { description: 'Input placeholder shown while no value is set.' },
    disabled: { description: 'Disables the input and hides the clear button.' },
  },
} satisfies Meta<typeof DateTimePicker>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    value: null,
    onChange: () => undefined,
    placeholder: 'Leave blank to never expire',
  },
};

export const ClientExpiry: Story = {
  args: { value: null, onChange: () => undefined },
  render: () => <ClientExpiryDemo />,
};

export const DateOnly: Story = {
  args: {
    value: dayjs('2026-08-01'),
    onChange: () => undefined,
    showTime: false,
    format: 'YYYY-MM-DD',
    placeholder: 'Start date',
  },
};

export const JalaliCalendar: Story = {
  args: { value: null, onChange: () => undefined },
  parameters: { docs: { disable: true } },
  render: () => <JalaliDemo />,
};
