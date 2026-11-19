import type { ReactNode } from 'react';
import type { AllSetting } from '@/models/setting';

export interface NotificationEventConfig {
  key: string;
  label: string;
  settingKey: string;
  hint?: string;
  extra?: (props: {
    value: number;
    onChange: (v: number | null) => void;
    ariaLabel: string;
    allSetting: AllSetting;
    updateSetting: (patch: Partial<AllSetting>) => void;
  }) => ReactNode;
}

export interface NotificationGroupConfig {
  icon: ReactNode;
  title: string;
  events: NotificationEventConfig[];
}
