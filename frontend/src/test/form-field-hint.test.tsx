import { screen } from '@testing-library/react';
import { FormProvider } from 'react-hook-form';
import { Form, InputNumber } from 'antd';
import { describe, expect, it } from 'vitest';
import { z } from 'zod';

import { FormField, useZodForm } from '@/components/form/rhf';

import { renderWithProviders } from './test-utils';

const Schema = z.object({ totalGB: z.number() });

function Harness() {
  const methods = useZodForm(Schema, { defaultValues: { totalGB: 0 } });
  return (
    <FormProvider {...methods}>
      <Form layout="vertical">
        <FormField name="totalGB" label="Total traffic" tooltip="0 means unlimited">
          <InputNumber />
        </FormField>
      </Form>
    </FormProvider>
  );
}

// The hint icon sat inside the <label>, so the field was named "Total traffic
// question-circle" and the hover-only hint never reached a screen reader (s-ui#1293).
describe('FormField tooltip hint', () => {
  it('keeps the icon out of the field name and exposes the hint as its description', () => {
    renderWithProviders(<Harness />);

    const field = screen.getByRole('spinbutton', { name: 'Total traffic' });
    const describedBy = field.getAttribute('aria-describedby') ?? '';
    expect(document.getElementById(describedBy)?.textContent).toBe('0 means unlimited');
  });
});
