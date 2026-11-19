import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, waitFor } from 'storybook/test';
import { Select } from 'antd';

import { selectTagRender } from './SelectTag';

const meta = {
  title: 'Form/SelectTag',
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Tag renderer for every `mode="tags"` / `mode="multiple"` Select. Ant Design draws the tag "x" as an aria-hidden, unfocusable span; `tagRender={selectTagRender}` swaps in a real button named "Remove <value>", so keyboard and screen-reader users can remove any tag, and focus moves to the next one.',
      },
    },
  },
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

function SniDemo() {
  const [value, setValue] = useState(['www.microsoft.com', 'www.apple.com', 'dl.google.com']);
  return (
    <div style={{ maxWidth: 420 }}>
      <Select
        aria-label="SNI"
        mode="tags"
        tagRender={selectTagRender}
        tokenSeparators={[',']}
        style={{ width: '100%' }}
        value={value}
        onChange={setValue}
      />
    </div>
  );
}

function InboundDemo() {
  const options = [
    { value: 1, label: 'VLESS Reality — 443' },
    { value: 2, label: 'VMess WS — 8443' },
    { value: 3, label: 'Trojan TCP — 2053' },
  ];
  const [value, setValue] = useState([1, 2, 3]);
  return (
    <div style={{ maxWidth: 420 }}>
      <Select
        aria-label="Inbounds"
        mode="multiple"
        tagRender={selectTagRender}
        style={{ width: '100%' }}
        options={options}
        value={value}
        onChange={setValue}
      />
    </div>
  );
}

export const FreeTextTags: Story = {
  render: () => <SniDemo />,
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();
    await userEvent.tab();
    await expect(canvas.getByRole('button', { name: 'Remove www.apple.com' })).toHaveFocus();
    await userEvent.keyboard('{Enter}');
    await expect(canvas.queryByText('www.apple.com')).toBeNull();
    await waitFor(() =>
      expect(canvas.getByRole('button', { name: 'Remove dl.google.com' })).toHaveFocus(),
    );
    await expect(canvas.getByRole('combobox', { name: 'SNI' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  },
};

export const OptionLabels: Story = {
  render: () => <InboundDemo />,
};
