import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Checkbox, Dropdown } from 'antd';
import { TableOutlined } from '@ant-design/icons';

import type { ClientColumnOption, HideableClientColumn } from './columnVisibility';

interface ClientColumnPickerProps {
  options: ClientColumnOption[];
  hidden: HideableClientColumn[];
  onChange: (hidden: HideableClientColumn[]) => void;
}

export default function ClientColumnPicker({ options, hidden, onChange }: ClientColumnPickerProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  const toggle = (key: string) => {
    const option = options.find((o) => o.key === key);
    if (!option) return;
    onChange(
      hidden.includes(option.key)
        ? hidden.filter((k) => k !== option.key)
        : [...hidden, option.key],
    );
  };

  return (
    <Dropdown
      trigger={['click']}
      open={open}
      autoFocus
      // Ticking a column must not close the menu; only the trigger or an outside click does.
      onOpenChange={(next, info) => {
        if (next || info.source === 'trigger') setOpen(next);
      }}
      menu={{
        items: options.map((o) => ({
          key: o.key,
          label: (
            <Checkbox
              checked={!hidden.includes(o.key)}
              tabIndex={-1}
              className="column-picker-check"
            >
              {o.label}
            </Checkbox>
          ),
        })),
        onClick: ({ key }) => toggle(key),
      }}
    >
      <Button icon={<TableOutlined aria-hidden />} aria-haspopup="menu" aria-expanded={open}>
        {t('pages.clients.columns')}
      </Button>
    </Dropdown>
  );
}
