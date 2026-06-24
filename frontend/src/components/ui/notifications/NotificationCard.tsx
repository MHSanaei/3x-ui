import type { ReactNode } from 'react';
import { Card } from 'antd';

interface Props {
  icon: ReactNode;
  title: ReactNode;
  extra: ReactNode;
  children: ReactNode;
}

export function NotificationCard({ icon, title, extra, children }: Props) {
  return (
    <Card
      size="small"
      bordered
      title={<span>{icon} {title}</span>}
      extra={extra}
      style={{ borderWidth: 1 }}
    >
      {children}
    </Card>
  );
}
