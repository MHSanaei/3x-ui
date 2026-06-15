import { useTranslation } from 'react-i18next';
import { Button } from 'antd';

interface SelectAllClearButtonsProps<T extends string | number = number> {
  options: Array<{ value: T }>;
  value: T[];
  onChange: (value: T[]) => void;
  /** Override the default "Select all" label (defaults to the inbound copy). */
  selectAllLabel?: string;
  /** Override the default "Clear all" label (defaults to the inbound copy). */
  clearLabel?: string;
}

export default function SelectAllClearButtons<T extends string | number = number>({
  options,
  value,
  onChange,
  selectAllLabel,
  clearLabel,
}: SelectAllClearButtonsProps<T>) {
  const { t } = useTranslation();

  const optionValues = options.map((o) => o.value);
  // Treat as "all selected" when every option is chosen, rather than comparing
  // lengths — this stays correct even if `value` holds ids outside `options`.
  const allSelected = options.length > 0 && optionValues.every((v) => value.includes(v));

  return (
    <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
      <Button
        size="small"
        disabled={allSelected}
        // Union with the current value so selections outside `options` are kept.
        onClick={() => onChange(Array.from(new Set([...value, ...optionValues])))}
      >
        {selectAllLabel ?? t('pages.clients.selectAllInbounds')}
      </Button>
      <Button
        size="small"
        disabled={value.length === 0}
        onClick={() => onChange([])}
      >
        {clearLabel ?? t('pages.clients.clearAllInbounds')}
      </Button>
    </div>
  );
}
