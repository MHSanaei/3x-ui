import { useEffect, useRef, useState } from 'react';
import { Form } from 'antd';

import { FinalMaskForm } from '@/lib/xray/forms/transport';
import type { FinalMaskStreamSettings } from '@/schemas/protocols/stream/finalmask';

interface FinalMaskFieldProps {
  value?: FinalMaskStreamSettings;
  onChange?: (next: FinalMaskStreamSettings) => void;
  network: string;
  protocol: string;
  showAll?: boolean;
}

const EMPTY: FinalMaskStreamSettings = { tcp: [], udp: [] };

export default function FinalMaskField({ value, onChange, network, protocol, showAll }: FinalMaskFieldProps) {
  const [form] = Form.useForm();
  const [initial] = useState(() => value ?? EMPTY);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const lastEmitted = useRef(JSON.stringify(initial));

  const finalmask = Form.useWatch('finalmask', form) as FinalMaskStreamSettings | undefined;

  useEffect(() => {
    if (finalmask === undefined) return;
    const serialized = JSON.stringify(finalmask);
    if (serialized === lastEmitted.current) return;
    lastEmitted.current = serialized;
    onChangeRef.current?.(finalmask);
  }, [finalmask]);

  return (
    <Form
      form={form}
      component={false}
      colon={false}
      labelCol={{ sm: { span: 8 } }}
      wrapperCol={{ sm: { span: 14 } }}
      labelWrap
      initialValues={{ finalmask: initial }}
    >
      <FinalMaskForm name="finalmask" network={network} protocol={protocol} form={form} showAll={showAll} />
    </Form>
  );
}
