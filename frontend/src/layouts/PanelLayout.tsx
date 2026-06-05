import { useEffect } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

import { useWebSocketBridge } from '@/api/websocketBridge';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useMe } from '@/hooks/useMe';

// Routes a non-admin "user" is allowed to render. Everything else is redirected
// to the clients page. This is a UX guard only — the backend independently
// enforces RBAC on every API, so a user who hand-types a URL or hits the API
// directly still gets nothing they are not entitled to.
const USER_ALLOWED_PATHS = new Set(['/clients', '/profile', '/billing']);

export default function PanelLayout() {
  useWebSocketBridge();
  usePageTitle();

  const { me } = useMe();
  const { pathname } = useLocation();
  const navigate = useNavigate();

  const restricted = !!me && !me.isAdmin && !USER_ALLOWED_PATHS.has(pathname);

  useEffect(() => {
    if (restricted) {
      navigate('/clients', { replace: true });
    }
  }, [restricted, navigate]);

  // Avoid flashing a forbidden page for the frame before the redirect lands.
  if (restricted) {
    return null;
  }

  return <Outlet />;
}
