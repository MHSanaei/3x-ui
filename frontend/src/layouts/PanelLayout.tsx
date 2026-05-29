import { Outlet } from 'react-router-dom';

import { useWebSocketBridge } from '@/api/websocketBridge';
import { usePageTitle } from '@/hooks/usePageTitle';

export default function PanelLayout() {
  useWebSocketBridge();
  usePageTitle();
  return <Outlet />;
}
