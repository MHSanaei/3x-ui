import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Form, Switch, type FormInstance } from 'antd';
import { useTranslation } from 'react-i18next';

import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

import { nestAtPath, parseJsonObject, serializeOverride } from './helpers';

interface OutboundSubtreeJsonFormProps {
  value?: string;
  onChange?: (next: string) => void;
  // Form path the inner form edits, e.g. ['streamSettings', 'sockopt'].
  path: (string | number)[];
  // Renders the reused outbound form given this wrapper's own form instance.
  render: (form: FormInstance<OutboundFormValues>) => ReactNode;
  // When true the wrapper owns an enable Switch (xhttp, which has no built-in
  // toggle); when false the inner form provides its own (sockopt's Switch).
  enableSwitch?: boolean;
  enableLabel?: string;
}

// Hosts the reused outbound transport forms (which bind to fixed
// streamSettings.* paths) inside an isolated antd Form, mirroring
// SubJsonFinalMaskForm: seed the inner form from the JSON string, watch the
// edited subtree, and report a pruned JSON string back to the parent host form.
export default function OutboundSubtreeJsonForm({
  value = '',
  onChange,
  path,
  render,
  enableSwitch = false,
  enableLabel,
}: OutboundSubtreeJsonFormProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [initial] = useState(() => parseJsonObject(value));
  const [enabled, setEnabled] = useState(() => value !== '');
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const subtree = Form.useWatch(path, form);

  useEffect(() => {
    if (enableSwitch && !enabled) return;
    const next = serializeOverride(subtree);
    if (next !== value) onChangeRef.current?.(next);
  }, [subtree, value, enabled, enableSwitch]);

  const hasInitial = Object.keys(initial).length > 0;
  const initialValues = nestAtPath(path, hasInitial ? initial : undefined);

  const toggle = (v: boolean) => {
    setEnabled(v);
    if (!v) onChangeRef.current?.('');
    else onChangeRef.current?.(serializeOverride(form.getFieldValue(path)));
  };

  return (
    <Form
      form={form}
      colon={false}
      labelCol={{ sm: { span: 8 } }}
      wrapperCol={{ sm: { span: 14 } }}
      labelWrap
      initialValues={initialValues}
    >
      {enableSwitch && (
        <Form.Item label={enableLabel ?? t('enable')}>
          <Switch checked={enabled} onChange={toggle} />
        </Form.Item>
      )}
      {(!enableSwitch || enabled) && render(form as unknown as FormInstance<OutboundFormValues>)}
    </Form>
  );
}
