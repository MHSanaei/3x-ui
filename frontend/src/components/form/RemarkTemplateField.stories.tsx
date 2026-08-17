import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import RemarkTemplateField from './RemarkTemplateField';

const meta = {
  title: 'Form/RemarkTemplateField',
  component: RemarkTemplateField,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Text input augmented with a {{VAR}} token picker (insert-at-caret) and a live sample-based preview of the expanded remark. The panel uses it in subscription settings for the global Remark Template.',
      },
    },
  },
  argTypes: {
    value: { description: 'Current template string; any {{VAR}} token enables the live preview below the input.' },
    onChange: { description: 'Called with the updated template on typing or token insertion.' },
    maxLength: { description: 'Maximum template length; picker insertions are clamped to it.' },
    placeholder: { description: 'Placeholder shown while the template is empty.' },
  },
} satisfies Meta<typeof RemarkTemplateField>;

export default meta;

type Story = StoryObj<typeof meta>;

function InteractiveDemo() {
  const [value, setValue] = useState('{{STATUS_EMOJI}} {{INBOUND}}-{{EMAIL}} | {{TRAFFIC_LEFT}}');
  return <RemarkTemplateField value={value} onChange={setValue} maxLength={256} placeholder="{{INBOUND}}-{{EMAIL}}" />;
}

export const Empty: Story = {
  args: {
    value: '',
    onChange: () => undefined,
    placeholder: '{{INBOUND}}-{{EMAIL}}',
  },
};

export const TokenTemplate: Story = {
  args: {
    value: '{{INBOUND}}-{{EMAIL}} | {{TRAFFIC_LEFT}} left | {{DAYS_LEFT}}d',
    onChange: () => undefined,
    maxLength: 256,
    placeholder: '{{INBOUND}}-{{EMAIL}}',
  },
};

export const PlainRemark: Story = {
  args: {
    value: 'Germany CDN node',
    onChange: () => undefined,
    maxLength: 256,
    placeholder: '{{INBOUND}}-{{EMAIL}}',
  },
};

export const Interactive: Story = {
  args: {
    value: '',
    onChange: () => undefined,
  },
  render: () => <InteractiveDemo />,
};
