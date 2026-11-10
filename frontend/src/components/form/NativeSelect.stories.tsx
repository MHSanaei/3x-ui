import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { NativeSelect } from './NativeSelect';

const meta = {
  title: 'Form/NativeSelect',
  component: NativeSelect,
  tags: ['autodocs'],
  parameters: {
    docs: {
      description: {
        component:
          'Native single selection for short, fixed lists. Labels, values, keyboard interaction and the mobile picker belong to the browser. Index keys keep number and string values distinct in onChange; searchable, clearable and multi-select fields continue using Ant Design.',
      },
    },
  },
  argTypes: {
    options: {
      description: 'Ordered labels, typed string/number values and optional disabled choices.',
    },
    value: {
      description: 'Controlled form value. Saved values absent from the list remain visible.',
    },
    onChange: {
      description: 'Receives the original typed option value for RHF or Ant Design Form.',
    },
    placeholder: { description: 'Shown until the form has a value.' },
    disabled: {
      description: 'Uses native disabled behavior and preserves the controlled form value.',
    },
    size: {
      description: 'Ant Design token size, without turning the control into a multi-row listbox.',
    },
  },
  args: {
    'aria-label': 'Protocol',
    options: [
      { value: 'vless', label: 'VLESS' },
      { value: 'shadowsocks', label: 'Shadowsocks' },
      { value: 'tuic', label: 'TUIC', disabled: true },
    ],
  },
} satisfies Meta<typeof NativeSelect>;
export default meta;
type Story = StoryObj<typeof meta>;

export const SingleSelection: Story = {
  render: function Render(args) {
    const [value, setValue] = useState<string | number>('vless');
    return <NativeSelect {...args} value={value} onChange={setValue} />;
  },
  play: async ({ canvas, userEvent }) => {
    const select = canvas.getByRole('combobox', { name: 'Protocol' });
    await expect(select).toHaveDisplayValue('VLESS');
    await userEvent.selectOptions(
      select,
      within(select).getByRole('option', { name: 'Shadowsocks' }),
    );
    await expect(select).toHaveDisplayValue('Shadowsocks');
    await expect(within(select).getByRole('option', { name: 'Shadowsocks' })).toHaveProperty(
      'selected',
      true,
    );
  },
};
