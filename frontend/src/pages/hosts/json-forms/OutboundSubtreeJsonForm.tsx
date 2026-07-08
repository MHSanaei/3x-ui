import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Form } from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import type { FieldValues } from 'react-hook-form';

import { nestAtPath, parseJsonObject, serializeOverride } from './helpers';

interface OutboundSubtreeJsonFormProps {
  value?: string;
  onChange?: (next: string) => void;
  /* Form path the inner form edits, e.g. ['streamSettings', 'sockopt'] or ['mux']. */
  path: (string | number)[];
  /* Renders the reused outbound form, which binds to this wrapper's RHF context. */
  render: () => ReactNode;
  /* Seeds the form when the stored value is empty, so toggling a section on
     pre-fills sensible defaults instead of blanks (used by Mux). */
  defaultSubtree?: Record<string, unknown>;
  /* Turns the edited subtree into the stored JSON string (default: prune empties).
     Mux overrides this to store '' (= inherit) when its enable flag is off. */
  serialize?: (subtree: unknown) => string;
}

/*
 * Hosts the reused outbound transport forms (which bind to fixed RHF paths)
 * inside an isolated RHF form, mirroring the sub-JSON adapters: seed the form
 * from the JSON string, watch the edited subtree, and report a JSON string back
 * to the parent host form. The antd Form is layout-only (component={false}
 * avoids a nested <form> DOM node); data binding runs through the RHF provider.
 */
export default function OutboundSubtreeJsonForm({
  value = '',
  onChange,
  path,
  render,
  defaultSubtree,
  serialize = serializeOverride,
}: OutboundSubtreeJsonFormProps) {
  const [initial] = useState<Record<string, unknown>>(() => {
    const parsed = parseJsonObject(value);
    return Object.keys(parsed).length ? parsed : (defaultSubtree ?? {});
  });
  const [defaultValues] = useState<FieldValues>(() => {
    const hasInitial = Object.keys(initial).length > 0;
    return nestAtPath(path, hasInitial ? initial : undefined) as FieldValues;
  });
  const methods = useForm<FieldValues>({ defaultValues });
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const subtree = useWatch({ control: methods.control, name: path.join('.') });

  useEffect(() => {
    const next = serialize(subtree);
    if (next !== value) onChangeRef.current?.(next);
    /* serialize is logically stable; re-run only when the edited subtree changes. */
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [subtree, value]);

  return (
    <FormProvider {...methods}>
      <Form
        component={false}
        colon={false}
        labelCol={{ sm: { span: 8 } }}
        wrapperCol={{ sm: { span: 14 } }}
        labelWrap
      >
        {render()}
      </Form>
    </FormProvider>
  );
}
