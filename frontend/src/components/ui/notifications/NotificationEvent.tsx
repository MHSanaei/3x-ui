import type { ReactNode } from 'react';
import { Checkbox } from 'antd';
import { useTranslation } from 'react-i18next';

interface Props {
  label: string;
  checked: boolean;
  onToggle: () => void;
  children?: ReactNode;
}

export function NotificationEvent({ label, checked, onToggle, children }: Props) {
  const { t } = useTranslation();
  return (
    <div>
      <Checkbox checked={checked} onChange={onToggle}>
        {t(label)}
      </Checkbox>
      {checked && children && (
        <div style={{ paddingLeft: 24, marginTop: 4 }}>
          {children}
        </div>
      )}
    </div>
  );
}
