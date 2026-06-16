import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Form, type FormInstance } from 'antd';

import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

import { nestAtPath, parseJsonObject, serializeOverride } from './helpers';

interface OutboundSubtreeJsonFormProps {
  value?: string;
  onChange?: (next: string) => void;
  // Form path the inner form edits, e.g. ['streamSettings', 'sockopt'] or ['mux'].
  path: (string | number)[];
  // Renders the reused outbound form given this wrapper's own form instance.
  render: (form: FormInstance<OutboundFormValues>) => ReactNode;
  // Seeds the form when the stored value is empty, so toggling a section on
  // pre-fills sensible defaults instead of blanks (used by Mux).
  defaultSubtree?: Record<string, unknown>;
  // Turns the edited subtree into the stored JSON string (default: prune empties).
  // Mux overrides this to store '' (= inherit) when its enable flag is off.
  serialize?: (subtree: unknown) => string;
}

// Hosts the reused outbound transport forms (which bind to fixed form paths)
// inside an isolated antd Form, mirroring SubJsonFinalMaskForm: seed the form
// from the JSON string, watch the edited subtree, and report a JSON string back
// to the parent host form. component={false} avoids a nested <form> DOM node.
export default function OutboundSubtreeJsonForm({
  value = '',
  onChange,
  path,
  render,
  defaultSubtree,
  serialize = serializeOverride,
}: OutboundSubtreeJsonFormProps) {
  const [form] = Form.useForm();
  const [initial] = useState<Record<string, unknown>>(() => {
    const parsed = parseJsonObject(value);
    return Object.keys(parsed).length ? parsed : (defaultSubtree ?? {});
  });
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const subtree = Form.useWatch(path, form);

  useEffect(() => {
    const next = serialize(subtree);
    if (next !== value) onChangeRef.current?.(next);
    // serialize is logically stable; re-run only when the edited subtree changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [subtree, value]);

  const hasInitial = Object.keys(initial).length > 0;
  const initialValues = nestAtPath(path, hasInitial ? initial : undefined);

  return (
    <Form
      form={form}
      component={false}
      colon={false}
      labelCol={{ sm: { span: 8 } }}
      wrapperCol={{ sm: { span: 14 } }}
      labelWrap
      initialValues={initialValues}
    >
      {render(form as unknown as FormInstance<OutboundFormValues>)}
    </Form>
  );
}
