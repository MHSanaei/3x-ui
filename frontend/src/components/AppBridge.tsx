import { useEffect } from 'react';
import { App } from 'antd';
import { setMessageInstance } from '@/utils/messageBus';

export default function AppBridge({ children }: { children: React.ReactNode }) {
  const { message } = App.useApp();
  useEffect(() => {
    setMessageInstance(message);
  }, [message]);
  return <>{children}</>;
}
