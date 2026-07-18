import { useEffect, useRef, useState } from 'react';
import { Form } from 'antd';

import SniffingFields from '@/lib/xray/forms/SniffingFields';
import { SniffingSchema, type Sniffing } from '@/schemas/primitives/sniffing';

interface SniffingFieldProps {
  value?: Sniffing;
  onChange?: (next: Sniffing) => void;
  enableLabel: string;
}

export default function SniffingField({ value, onChange, enableLabel }: SniffingFieldProps) {
  const [form] = Form.useForm();
  const [initial] = useState(() => value ?? SniffingSchema.parse({}));
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const lastEmitted = useRef(JSON.stringify(initial));

  const sniffing = Form.useWatch('sniffing', { form, preserve: true }) as Sniffing | undefined;

  useEffect(() => {
    if (sniffing === undefined) return;
    const serialized = JSON.stringify(sniffing);
    if (serialized === lastEmitted.current) return;
    lastEmitted.current = serialized;
    onChangeRef.current?.(sniffing);
  }, [sniffing]);

  useEffect(() => {
    if (value === undefined) return;
    const serialized = JSON.stringify(value);
    if (serialized === lastEmitted.current) return;
    lastEmitted.current = serialized;
    form.setFieldsValue({ sniffing: value });
  }, [value, form]);

  return (
    <Form
      form={form}
      component={false}
      colon={false}
      labelCol={{ sm: { span: 8 } }}
      wrapperCol={{ sm: { span: 14 } }}
      labelWrap
      initialValues={{ sniffing: initial }}
    >
      <SniffingFields name={['sniffing']} form={form} enableLabel={enableLabel} />
    </Form>
  );
}
