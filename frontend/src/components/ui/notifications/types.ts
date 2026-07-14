import type { ReactNode } from 'react';

export interface NotificationEventConfig {
  key: string;
  label: string;
  settingKey: string;
  extra?: (props: { value: number; onChange: (v: number | null) => void; ariaLabel: string }) => ReactNode;
}

export interface NotificationGroupConfig {
  icon: ReactNode;
  title: string;
  events: NotificationEventConfig[];
}
