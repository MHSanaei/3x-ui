import { Select } from 'antd';
import type { SelectProps } from 'antd';

import { TLS_CIPHER_OPTION } from '@/schemas/primitives';

const CIPHER_SUITE_OPTIONS = Object.values(TLS_CIPHER_OPTION).map((v) => ({ value: v, label: v }));

type CipherSuitesSelectProps = Omit<
  SelectProps<string[]>,
  'value' | 'onChange' | 'mode' | 'options'
> & {
  // Injected by FormField:
  value?: string;
  onChange?: (value: string) => void;
};

// xray splits cipherSuites on ':' into a list, so the picker edits tags while
// the stored value stays the single colon-joined string xray reads.
export default function CipherSuitesSelect({
  value = '',
  onChange,
  ...rest
}: CipherSuitesSelectProps) {
  const suites = value
    .split(':')
    .map((s) => s.trim())
    .filter(Boolean);
  return (
    <Select
      allowClear
      tokenSeparators={[':', ',']}
      {...rest}
      mode="tags"
      options={CIPHER_SUITE_OPTIONS}
      value={suites}
      onChange={(next) => onChange?.(next.join(':'))}
    />
  );
}
