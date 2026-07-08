import { Button, Input, Space } from 'antd';
import { useTranslation } from 'react-i18next';

interface SecretInputProps {
  value: string;
  configured: boolean;
  clearArmed: boolean;
  placeholder: string;
  onChange: (value: string) => void;
  onClearArmedChange: (armed: boolean) => void;
}

export default function SecretInput({
  value,
  configured,
  clearArmed,
  placeholder,
  onChange,
  onClearArmedChange,
}: SecretInputProps) {
  const { t } = useTranslation();
  return (
    <Space.Compact style={{ width: '100%' }}>
      <Input.Password
        value={value}
        placeholder={configured && !clearArmed ? placeholder : ''}
        onChange={(e) => {
          onChange(e.target.value);
          if (clearArmed) onClearArmedChange(false);
        }}
      />
      {configured && (
        <Button
          danger={clearArmed}
          onClick={() => {
            onChange('');
            onClearArmedChange(!clearArmed);
          }}
        >
          {clearArmed ? t('pages.settings.secretClearUndo') : t('pages.settings.secretClear')}
        </Button>
      )}
    </Space.Compact>
  );
}
