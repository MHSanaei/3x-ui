import { Space } from 'antd';
import { useTranslation } from 'react-i18next';
import type { AllSetting } from '@/models/setting';
import type { NotificationGroupConfig } from './types';
import { NotificationCard } from './NotificationCard';
import { NotificationHeader } from './NotificationHeader';
import { NotificationEvent } from './NotificationEvent';

interface Props {
  config: NotificationGroupConfig;
  selected: string[];
  onToggle: (key: string) => void;
  onToggleAll: (keys: string[]) => void;
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export function NotificationGroup({ config, selected, onToggle, onToggleAll, allSetting, updateSetting }: Props) {
  const { t } = useTranslation();

  const count = config.events.filter((e) => selected.includes(e.key)).length;
  const total = config.events.length;

  function toggleAll() {
    const values = config.events.map((e) => e.key);
    onToggleAll(values);
  }

  return (
    <NotificationCard
      icon={config.icon}
      title={t(`pages.settings.${config.title}`)}
      extra={
        <NotificationHeader
          count={count}
          total={total}
          allSelected={count === total}
          indeterminate={count > 0 && count < total}
          onToggleAll={toggleAll}
        />
      }
    >
      <Space orientation="vertical" size={8} style={{ width: '100%' }}>
        {config.events.map((event) => (
          <NotificationEvent
            key={event.key}
            label={t(`pages.settings.${event.label}`)}
            checked={selected.includes(event.key)}
            onToggle={() => onToggle(event.key)}
          >
            {event.extra?.({
              value: Number((allSetting as unknown as Record<string, unknown>)[event.settingKey]) || 0,
              onChange: (v) => updateSetting({ [event.settingKey]: v }),
              ariaLabel: t(`pages.settings.${event.label}`),
            })}
          </NotificationEvent>
        ))}
      </Space>
    </NotificationCard>
  );
}
