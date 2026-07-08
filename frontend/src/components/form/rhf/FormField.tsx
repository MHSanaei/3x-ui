import { cloneElement } from 'react';
import type { CSSProperties, ReactElement, ReactNode } from 'react';
import { Controller } from 'react-hook-form';
import type { Control, ControllerProps, FieldValues, Path } from 'react-hook-form';
import { Form } from 'antd';
import { useTranslation } from 'react-i18next';

import { normalizeAntdOnChange, type ValueProp } from './normalizeAntdOnChange';
import { toDotted, type FieldName } from './toDotted';

interface FormFieldTransform {
  input?: (value: unknown) => unknown;
  output?: (eventValue: unknown) => unknown;
}

export interface FormFieldProps<T extends FieldValues = FieldValues> {
  name: FieldName;
  control?: Control<T>;
  label?: ReactNode;
  tooltip?: ReactNode;
  extra?: ReactNode;
  valueProp?: ValueProp;
  transform?: FormFieldTransform;
  onAfterChange?: (value: unknown) => void;
  rules?: ControllerProps<T>['rules'];
  required?: boolean;
  noStyle?: boolean;
  className?: string;
  style?: CSSProperties;
  children: ReactElement;
}

export function FormField<T extends FieldValues = FieldValues>({
  name,
  control,
  label,
  tooltip,
  extra,
  valueProp = 'value',
  transform,
  onAfterChange,
  rules,
  required,
  noStyle,
  className,
  style,
  children,
}: FormFieldProps<T>) {
  const { t } = useTranslation();
  const dottedName = toDotted(name) as Path<T>;

  return (
    <Controller
      control={control}
      name={dottedName}
      rules={rules}
      render={({ field, fieldState }) => {
        const displayValue = transform?.input ? transform.input(field.value) : field.value;
        const help = fieldState.error?.message
          ? t(fieldState.error.message, { defaultValue: fieldState.error.message })
          : undefined;
        const childProps: Record<string, unknown> = {
          [valueProp]: displayValue,
          onChange: (...args: unknown[]) => {
            const raw = normalizeAntdOnChange(args, valueProp);
            const next = transform?.output ? transform.output(raw) : raw;
            field.onChange(next);
            onAfterChange?.(next);
          },
          onBlur: field.onBlur,
          ref: field.ref,
        };
        return (
          <Form.Item
            label={label}
            tooltip={tooltip}
            extra={extra}
            required={required}
            validateStatus={fieldState.error ? 'error' : undefined}
            help={help}
            noStyle={noStyle}
            className={className}
            style={style}
          >
            {cloneElement(children as ReactElement<Record<string, unknown>>, childProps)}
          </Form.Item>
        );
      }}
    />
  );
}

export default FormField;
