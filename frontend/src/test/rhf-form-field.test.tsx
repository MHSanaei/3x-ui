import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { FormProvider } from 'react-hook-form';
import { Form, Input, InputNumber, Switch } from 'antd';
import { z } from 'zod';

import { FormField, useZodForm } from '@/components/form/rhf';

const GB = 1024 * 1024 * 1024;

const Schema = z.object({
  email: z.string().min(1, 'email-required'),
  enabled: z.boolean(),
  slug: z.string(),
  bytes: z.number(),
});

type Values = z.infer<typeof Schema>;

function Harness({ onSubmit }: { onSubmit: (values: Values) => void }) {
  const methods = useZodForm(Schema, {
    defaultValues: { email: '', enabled: false, slug: '', bytes: 2 * GB },
  });
  return (
    <FormProvider {...methods}>
      <Form layout="vertical">
        <FormField name="email" label="Email">
          <Input aria-label="email" />
        </FormField>
        <FormField name="enabled" label="Enabled" valueProp="checked">
          <Switch aria-label="enabled" />
        </FormField>
        <FormField name="slug" label="Slug" transform={{ output: (v) => String(v).toLowerCase() }}>
          <Input aria-label="slug" />
        </FormField>
        <FormField
          name="bytes"
          label="Traffic"
          transform={{
            input: (v) => (typeof v === 'number' ? v / GB : v),
            output: (v) => (typeof v === 'number' ? v * GB : 0),
          }}
        >
          <InputNumber aria-label="bytes" />
        </FormField>
        <button type="button" onClick={methods.handleSubmit(onSubmit)}>Save</button>
      </Form>
    </FormProvider>
  );
}

describe('FormField', () => {
  it('submits values normalized from the Ant Design inputs', async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} />);

    fireEvent.change(screen.getByLabelText('email'), { target: { value: 'a@b.com' } });
    fireEvent.click(screen.getByLabelText('enabled'));
    fireEvent.change(screen.getByLabelText('slug'), { target: { value: 'HELLO' } });
    fireEvent.click(screen.getByText('Save'));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      email: 'a@b.com',
      enabled: true,
      slug: 'hello',
      bytes: 2 * GB,
    });
  });

  it('applies the transform output before storing the value', async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} />);

    fireEvent.change(screen.getByLabelText('email'), { target: { value: 'x' } });
    const bytes = screen.getByLabelText('bytes');
    fireEvent.change(bytes, { target: { value: '5' } });
    fireEvent.blur(bytes);
    fireEvent.click(screen.getByText('Save'));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0][0].bytes).toBe(5 * GB);
  });

  it('surfaces a resolver validation error as help text and blocks submit', async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} />);

    fireEvent.click(screen.getByText('Save'));

    await screen.findByText('email-required');
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
