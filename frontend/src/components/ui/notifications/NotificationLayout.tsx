import type { ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

export function NotificationLayout({ children }: Props) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 12 }}>
      {children}
    </div>
  );
}
