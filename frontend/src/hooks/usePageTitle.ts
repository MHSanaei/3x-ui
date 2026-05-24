import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

const TITLES: Record<string, string> = {
  '/': 'Overview',
  '/inbounds': 'Inbounds',
  '/clients': 'Clients',
  '/nodes': 'Nodes',
  '/settings': 'Settings',
  '/xray': 'Xray Config',
  '/api-docs': 'API Docs',
};

export function usePageTitle() {
  const { pathname } = useLocation();

  useEffect(() => {
    const title = TITLES[pathname] || '3X-UI';
    const host = window.location.hostname;
    document.title = host ? `${host} - ${title}` : title;
  }, [pathname]);
}
