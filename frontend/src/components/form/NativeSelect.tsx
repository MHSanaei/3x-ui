import type { ComponentPropsWithRef } from 'react';
import { theme } from 'antd';

import './NativeSelect.css';

export interface NativeSelectProps<Value extends string | number> extends Omit<
  ComponentPropsWithRef<'select'>,
  'value' | 'defaultValue' | 'onChange' | 'multiple' | 'children' | 'size'
> {
  value?: Value | null;
  onChange?: (value: Value) => void;
  options: readonly { value: Value; label: string | number; disabled?: boolean }[];
  placeholder?: string;
  size?: 'small' | 'middle' | 'large';
}

export function NativeSelect<Value extends string | number>({
  value,
  options,
  onChange,
  placeholder,
  disabled,
  className,
  style,
  size,
  ...props
}: NativeSelectProps<Value>) {
  const { token } = theme.useToken();
  const selected = options.findIndex((option) => Object.is(option.value, value));
  const hasUnlistedValue = selected < 0 && value != null;
  return (
    <select
      {...props}
      className={['native-select', className].filter(Boolean).join(' ')}
      disabled={disabled}
      value={selected < 0 ? '' : String(selected)}
      onChange={(event) => {
        const option = options[Number(event.target.value)];
        if (event.target.value !== '' && option && !option.disabled) onChange?.(option.value);
      }}
      style={{
        width: '100%',
        minWidth: 0,
        height:
          size === 'small'
            ? token.controlHeightSM
            : size === 'large'
              ? token.controlHeightLG
              : token.controlHeight,
        border: `1px solid ${props['aria-invalid'] === true || props['aria-invalid'] === 'true' ? token.colorError : token.colorBorder}`,
        borderRadius: token.borderRadius,
        backgroundColor: disabled ? token.colorBgContainerDisabled : token.colorBgContainer,
        color: disabled ? token.colorTextDisabled : token.colorText,
        fontSize: token.fontSize,
        ...style,
      }}
    >
      {selected < 0 && (
        <option value="" disabled={!hasUnlistedValue} hidden={!hasUnlistedValue}>
          {hasUnlistedValue ? String(value) : (placeholder ?? '')}
        </option>
      )}
      {options.map((option, index) => (
        <option key={String(index)} value={String(index)} disabled={option.disabled}>
          {option.label}
        </option>
      ))}
    </select>
  );
}
