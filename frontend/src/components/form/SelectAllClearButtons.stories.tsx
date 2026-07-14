import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect } from 'storybook/test';
import { Select } from 'antd';

import SelectAllClearButtons from './SelectAllClearButtons';

const inboundOptions: Array<{ value: number; label: string }> = [
  { value: 1, label: 'VLESS Reality — 443' },
  { value: 2, label: 'VMess WS — 8443' },
  { value: 3, label: 'Trojan TCP — 2053' },
  { value: 4, label: 'Shadowsocks — 8388' },
];

const clientEmailOptions: Array<{ value: string; label: string }> = [
  { value: 'ava@corp.example', label: 'ava@corp.example' },
  { value: 'reza.mobile', label: 'reza.mobile' },
  { value: 'office-tv', label: 'office-tv' },
  { value: 'guest-42', label: 'guest-42' },
];

const meta = {
  title: 'Form/SelectAllClearButtons',
  component: SelectAllClearButtons,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Small "Select all" / "Clear all" button pair rendered above a multi-select. The panel places it over the attached-inbounds picker in the client form and the bulk attach/detach modals.',
      },
    },
  },
  argTypes: {
    options: { description: 'Option list whose values define the "all" set; matches the AntD Select option shape.' },
    value: { description: 'Currently selected values (controlled).' },
    onChange: { description: 'Called with the union of the current selection and every option value, or with an empty array on clear.' },
    selectAllLabel: { description: 'Override for the "Select all" button text; defaults to the translated inbound copy.' },
    clearLabel: { description: 'Override for the "Clear all" button text; defaults to the translated inbound copy.' },
  },
} satisfies Meta<typeof SelectAllClearButtons>;

export default meta;

type Story = StoryObj<typeof meta>;

function InboundPickerDemo() {
  const [selected, setSelected] = useState<number[]>([1]);
  return (
    <div style={{ maxWidth: 360 }}>
      <SelectAllClearButtons options={inboundOptions} value={selected} onChange={setSelected} />
      <Select
        mode="multiple"
        style={{ width: '100%' }}
        value={selected}
        onChange={setSelected}
        options={inboundOptions}
        placeholder="Select inbounds"
        aria-label="Select inbounds"
        maxTagCount="responsive"
      />
    </div>
  );
}

function ClientEmailsDemo() {
  const [selected, setSelected] = useState<string[]>(['ava@corp.example']);
  return (
    <div style={{ maxWidth: 360 }}>
      <SelectAllClearButtons
        options={clientEmailOptions}
        value={selected}
        onChange={setSelected}
        selectAllLabel="Select all clients"
        clearLabel="Deselect clients"
      />
      <Select
        mode="multiple"
        style={{ width: '100%' }}
        value={selected}
        onChange={setSelected}
        options={clientEmailOptions}
        placeholder="Select clients"
        aria-label="Select clients"
        maxTagCount="responsive"
      />
    </div>
  );
}

const placeholderArgs = {
  options: [],
  value: [],
  onChange: () => undefined,
};

export const PartiallySelected: Story = {
  args: {
    options: [{ value: 1 }, { value: 2 }, { value: 3 }, { value: 4 }],
    value: [1, 3],
    onChange: () => undefined,
  },
};

export const AllSelected: Story = {
  args: {
    options: [{ value: 1 }, { value: 2 }, { value: 3 }],
    value: [1, 2, 3],
    onChange: () => undefined,
  },
};

export const WithInboundSelect: Story = {
  args: placeholderArgs,
  render: () => <InboundPickerDemo />,
};

export const CustomLabels: Story = {
  args: placeholderArgs,
  render: () => <ClientEmailsDemo />,
  play: async ({ canvas, userEvent }) => {
    const selectAll = canvas.getByRole('button', { name: 'Select all clients' });
    const clearAll = canvas.getByRole('button', { name: 'Deselect clients' });
    await expect(selectAll).toBeEnabled();
    await userEvent.click(selectAll);
    await expect(selectAll).toBeDisabled();
    await userEvent.click(clearAll);
    await expect(clearAll).toBeDisabled();
    await expect(selectAll).toBeEnabled();
  },
};
