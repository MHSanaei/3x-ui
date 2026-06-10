import { useTranslation } from 'react-i18next';
import { Button } from 'antd';

interface Option {
  value: number;
}

interface SelectAllClearButtonsProps {
  options: Option[];
  value: number[];
  onChange: (value: number[]) => void;
}

export default function SelectAllClearButtons({
  options,
  value,
  onChange,
}: SelectAllClearButtonsProps) {
  const { t } = useTranslation();

  return (
    <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
      <Button
        size="small"
        disabled={options.length === 0 || value.length === options.length}
        onClick={() => onChange(options.map((o) => o.value))}
      >
        {t('pages.clients.selectAllInbounds')}
      </Button>
      <Button
        size="small"
        disabled={value.length === 0}
        onClick={() => onChange([])}
      >
        {t('pages.clients.clearAllInbounds')}
      </Button>
    </div>
  );
}
