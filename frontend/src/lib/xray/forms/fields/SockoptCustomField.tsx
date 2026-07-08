import { useEffect, useRef, useState } from 'react';
import { Form } from 'antd';

import { CustomSockoptList } from '@/components/form';
import type { CustomSockopt } from '@/schemas/protocols/stream/sockopt';

interface SockoptCustomFieldProps {
  value?: CustomSockopt[];
  onChange?: (next: CustomSockopt[]) => void;
}

export default function SockoptCustomField({ value, onChange }: SockoptCustomFieldProps) {
  const [form] = Form.useForm();
  const [initial] = useState(() => value ?? []);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const lastEmitted = useRef(JSON.stringify(initial));

  const list = Form.useWatch('customSockopt', form) as CustomSockopt[] | undefined;

  useEffect(() => {
    if (list === undefined) return;
    const serialized = JSON.stringify(list);
    if (serialized === lastEmitted.current) return;
    lastEmitted.current = serialized;
    onChangeRef.current?.(list);
  }, [list]);

  return (
    <Form form={form} component={false} initialValues={{ customSockopt: initial }}>
      <CustomSockoptList name={['customSockopt']} />
    </Form>
  );
}
