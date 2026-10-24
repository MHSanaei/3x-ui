import { Suspense } from 'react';
import { createBrowserRouter, type RouteObject } from 'react-router';
import { Spin } from 'antd';

import PanelLayout from '@/layouts/PanelLayout';
import { PANEL_ROUTES } from '@/routes/panelRoutes';

function withSuspense(node: React.ReactNode) {
  return (
    <Suspense
      fallback={
        <div
          style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: '60vh',
          }}
        >
          <Spin size="large" />
        </div>
      }
    >
      {node}
    </Suspense>
  );
}

export const routes: RouteObject[] = [
  {
    path: '/',
    element: <PanelLayout />,
    children: PANEL_ROUTES.map((route) => {
      const element = withSuspense(<route.component />);
      return route.path === '/' ? { index: true, element } : { path: route.path.slice(1), element };
    }),
  },
];

function computeBasename() {
  const raw = (typeof window !== 'undefined' && window.X_UI_BASE_PATH) || '/';
  const trimmed = raw.replace(/\/+$/, '');
  return `${trimmed}/panel`;
}

export const router = createBrowserRouter(routes, {
  basename: computeBasename(),
});
