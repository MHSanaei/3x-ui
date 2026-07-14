import { useEffect } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, waitFor } from 'storybook/test';
import { Button, Form, Input, InputNumber, Select, Switch, Typography } from 'antd';
import { FormProvider } from 'react-hook-form';
import { z } from 'zod';

import { FormField } from './FormField';
import { useZodForm } from './useZodForm';

const GB = 1024 * 1024 * 1024;

const meta = {
  title: 'Form/RHF/FormField',
  component: FormField,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'Bridges one Ant Design control into react-hook-form: wraps the child in a Controller plus Form.Item, normalizes the onChange payload, and surfaces resolver errors as translated help text. Every RHF panel form (client, inbound, outbound, and host modals) builds its fields with it.',
      },
    },
  },
  argTypes: {
    name: { description: 'Field path — a dotted string or an array of segments joined with dots.' },
    control: { description: 'Optional react-hook-form control; falls back to the surrounding FormProvider.' },
    label: { description: 'Form.Item label.' },
    tooltip: { description: 'Form.Item tooltip shown next to the label.' },
    extra: { description: 'Helper text rendered below the input.' },
    valueProp: { description: 'Prop the child receives the value on: `value` (default) or `checked` for switches.' },
    transform: { description: 'Optional input/output mappers, e.g. bytes stored in the form but GB shown in the input.' },
    onAfterChange: { description: 'Called with the stored value after every change.' },
    rules: { description: 'Controller-level validation rules applied on top of the form resolver.' },
    required: { description: 'Marks the label with the required asterisk.' },
    noStyle: { description: 'Render the bare input without Form.Item chrome.' },
    children: { description: 'The single Ant Design control to wire up.' },
  },
} satisfies Meta<typeof FormField>;

export default meta;

type Story = StoryObj<typeof meta>;

const ClientSchema = z.object({
  email: z.string(),
  flow: z.string(),
  enable: z.boolean(),
});

function ClientDemo() {
  const methods = useZodForm(ClientSchema, {
    defaultValues: { email: 'user1@example.com', flow: 'xtls-rprx-vision', enable: true },
  });
  return (
    <FormProvider {...methods}>
      <Form layout="vertical" style={{ maxWidth: 360 }}>
        <FormField name="email" label="Email" tooltip="Unique identifier used to match client traffic" required>
          <Input placeholder="user1@example.com" />
        </FormField>
        <FormField name="flow" label="Flow" extra="Only applies to VLESS over raw TLS">
          <Select
            options={[
              { label: 'none', value: '' },
              { label: 'xtls-rprx-vision', value: 'xtls-rprx-vision' },
            ]}
          />
        </FormField>
        <FormField name="enable" label="Enable" valueProp="checked">
          <Switch />
        </FormField>
      </Form>
    </FormProvider>
  );
}

const TrafficSchema = z.object({
  totalBytes: z.number(),
});

function TrafficDemo() {
  const methods = useZodForm(TrafficSchema, { defaultValues: { totalBytes: 50 * GB } });
  const totalBytes = methods.watch('totalBytes');
  return (
    <FormProvider {...methods}>
      <Form layout="vertical" style={{ maxWidth: 360 }}>
        <FormField
          name="totalBytes"
          label="Total traffic (GB)"
          extra="Stored on the client as bytes; 0 means unlimited"
          transform={{
            input: (value) => (typeof value === 'number' ? value / GB : value),
            output: (value) => (typeof value === 'number' ? value * GB : 0),
          }}
        >
          <InputNumber min={0} style={{ width: '100%' }} />
        </FormField>
        <Typography.Text type="secondary">Form state: {totalBytes.toLocaleString()} bytes</Typography.Text>
      </Form>
    </FormProvider>
  );
}

const InboundSchema = z.object({
  remark: z.string().min(1, 'Remark is required'),
  port: z
    .number()
    .min(1, 'Port must be between 1 and 65535')
    .max(65535, 'Port must be between 1 and 65535'),
});

function ValidationDemo() {
  const methods = useZodForm(InboundSchema, { defaultValues: { remark: '', port: 0 } });
  useEffect(() => {
    void methods.trigger();
  }, [methods]);
  return (
    <FormProvider {...methods}>
      <Form layout="vertical" style={{ maxWidth: 360 }}>
        <FormField name="remark" label="Remark" required>
          <Input placeholder="vless-reality-443" />
        </FormField>
        <FormField name="port" label="Port" required>
          <InputNumber style={{ width: '100%' }} />
        </FormField>
        <Button onClick={() => void methods.trigger()}>Validate</Button>
      </Form>
    </FormProvider>
  );
}

const RealitySchema = z.object({
  streamSettings: z.object({
    realitySettings: z.object({
      dest: z.string(),
      serverNames: z.string(),
    }),
  }),
});

function NestedDemo() {
  const methods = useZodForm(RealitySchema, {
    defaultValues: {
      streamSettings: {
        realitySettings: { dest: 'yahoo.com:443', serverNames: 'yahoo.com,www.yahoo.com' },
      },
    },
  });
  return (
    <FormProvider {...methods}>
      <Form layout="vertical" style={{ maxWidth: 360 }}>
        <FormField
          name={['streamSettings', 'realitySettings', 'dest']}
          label="Dest"
          tooltip="Camouflage target the REALITY handshake is proxied to"
        >
          <Input />
        </FormField>
        <FormField
          name="streamSettings.realitySettings.serverNames"
          label="Server names"
          extra="Comma-separated SNI list offered to clients"
        >
          <Input />
        </FormField>
      </Form>
    </FormProvider>
  );
}

const placeholderArgs = {
  name: 'email',
  children: <Input />,
};

export const ClientFields: Story = {
  args: placeholderArgs,
  render: () => <ClientDemo />,
};

export const TrafficTransform: Story = {
  args: placeholderArgs,
  render: () => <TrafficDemo />,
  play: async ({ canvas, userEvent }) => {
    const input = canvas.getByRole('spinbutton');
    await userEvent.clear(input);
    await userEvent.type(input, '100');
    await expect(await canvas.findByText(/107,374,182,400 bytes/)).toBeVisible();
  },
};

export const ValidationErrors: Story = {
  args: placeholderArgs,
  render: () => <ValidationDemo />,
  play: async ({ canvas, userEvent }) => {
    const remarkError = await canvas.findByText('Remark is required');
    await waitFor(() => expect(remarkError).toBeVisible());
    await waitFor(() => expect(canvas.getByText('Port must be between 1 and 65535')).toBeVisible());
    await userEvent.type(canvas.getByPlaceholderText('vless-reality-443'), 'vless-reality-443');
    await userEvent.click(canvas.getByRole('button', { name: 'Validate' }));
    await waitFor(() => expect(canvas.queryByText('Remark is required')).not.toBeInTheDocument());
    await expect(canvas.getByText('Port must be between 1 and 65535')).toBeVisible();
  },
};

export const NestedNames: Story = {
  args: placeholderArgs,
  render: () => <NestedDemo />,
};
