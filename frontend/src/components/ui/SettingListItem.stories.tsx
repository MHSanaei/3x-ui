import type { Meta, StoryObj } from '@storybook/react-vite';
import { InputNumber, Switch } from 'antd';

import SettingListItem from './SettingListItem';

const meta = {
  title: 'UI/SettingListItem',
  component: SettingListItem,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Two-column settings row: a title and description on the left, and a control (Switch, InputNumber, …) on the right. Associates the title with the control via `aria-labelledby` for accessibility.',
      },
    },
  },
  argTypes: {
    title: { description: 'Setting name shown on the left.' },
    description: { description: 'Secondary help text under the title.' },
    control: { description: 'The control rendered on the right (Switch, InputNumber, …).' },
    children: { description: 'Alternative to `control`; used when no explicit control is passed.' },
    paddings: { description: 'Row density: `default` or the tighter `small`.' },
  },
} satisfies Meta<typeof SettingListItem>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithSwitch: Story = {
  args: {
    title: 'Enable subscription',
    description: 'Expose an aggregated subscription URL for this client.',
    control: <Switch defaultChecked />,
  },
};

export const WithNumber: Story = {
  args: {
    title: 'Traffic limit',
    description: 'Cap total traffic in gigabytes. Zero means unlimited.',
    control: <InputNumber min={0} defaultValue={100} style={{ width: '100%' }} />,
  },
};

export const CompactPadding: Story = {
  args: {
    paddings: 'small',
    title: 'Auto renew',
    description: 'Restart the quota window automatically on depletion.',
    control: <Switch />,
  },
};
