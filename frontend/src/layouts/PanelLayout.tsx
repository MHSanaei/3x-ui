import { Outlet } from 'react-router-dom';

import { useWebSocketBridge } from '@/api/websocketBridge';

export default function PanelLayout() {
  useWebSocketBridge();
  return <Outlet />;
}
