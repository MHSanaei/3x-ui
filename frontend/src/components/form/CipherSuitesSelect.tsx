import { Select } from 'antd';
import type { SelectProps } from 'antd';
import { useTranslation } from 'react-i18next';

import { TLS_CIPHER_OPTION, TLS_CIPHER_PRESETS } from '@/schemas/primitives';

const CIPHER_SUITE_OPTIONS = Object.values(TLS_CIPHER_OPTION).map((v) => ({ value: v, label: v }));

const PRESET_PREFIX = 'preset:';
const PRESET_OPTIONS = Object.keys(TLS_CIPHER_PRESETS).map((name) => ({
  value: PRESET_PREFIX + name,
  label: name,
}));

type CipherSuitesSelectProps = Omit<
  SelectProps<string[]>,
  'value' | 'onChange' | 'mode' | 'options'
> & {
  // Injected by FormField:
  value?: string;
  onChange?: (value: string) => void;
};

function expandPreset(next: string[]): string[] {
  const picked = [...next].reverse().find((v) => v.startsWith(PRESET_PREFIX));
  if (!picked) return next;
  const name = picked.slice(PRESET_PREFIX.length) as keyof typeof TLS_CIPHER_PRESETS;
  return TLS_CIPHER_PRESETS[name].split(':');
}

// xray splits cipherSuites on ':' into a list, so the picker edits tags while
// the stored value stays the single colon-joined string xray reads.
export default function CipherSuitesSelect({
  value = '',
  onChange,
  ...rest
}: CipherSuitesSelectProps) {
  const { t } = useTranslation();
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
      options={[
        { label: t('pages.inbounds.form.presets'), options: PRESET_OPTIONS },
        { label: t('pages.inbounds.form.cipherSuites'), options: CIPHER_SUITE_OPTIONS },
      ]}
      value={suites}
      onChange={(next) => onChange?.(expandPreset(next).join(':'))}
    />
  );
}
