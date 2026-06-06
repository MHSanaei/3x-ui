import { useEffect, useRef, useState } from 'react';
import { Form } from 'antd';

import { FinalMaskForm } from '@/lib/xray/forms/transport';
import type { FinalMaskStreamSettings } from '@/schemas/protocols/stream/finalmask';

interface SubJsonFinalMaskFormProps {
  value: string;
  onChange: (next: string) => void;
}

function hasValue(v: unknown): boolean {
  if (v == null) return false;
  if (Array.isArray(v)) return v.some(hasValue);
  if (typeof v === 'object') return Object.values(v as Record<string, unknown>).some(hasValue);
  if (typeof v === 'string') return v.length > 0;
  return true;
}

function parseFinalMask(raw: string): FinalMaskStreamSettings {
  try {
    if (raw) return JSON.parse(raw) as FinalMaskStreamSettings;
  } catch {
    return { tcp: [], udp: [] };
  }
  return { tcp: [], udp: [] };
}

export default function SubJsonFinalMaskForm({ value, onChange }: SubJsonFinalMaskFormProps) {
  const [form] = Form.useForm();
  const [initial] = useState(() => parseFinalMask(value));
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const finalmask = Form.useWatch('finalmask', form) as FinalMaskStreamSettings | undefined;

  useEffect(() => {
    if (finalmask === undefined) return;
    const next = hasValue(finalmask) ? JSON.stringify(finalmask) : '';
    if (next !== value) onChangeRef.current(next);
  }, [finalmask, value]);

  return (
    <Form
      form={form}
      layout="horizontal"
      labelCol={{ flex: '160px' }}
      wrapperCol={{ flex: 'auto' }}
      colon={false}
      initialValues={{ finalmask: initial }}
    >
      <FinalMaskForm name="finalmask" network="" protocol="" form={form} showAll />
    </Form>
  );
}
