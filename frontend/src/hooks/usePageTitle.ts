import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

const TITLE_KEYS: Record<string, string> = {
  '/': 'menu.dashboard',
  '/inbounds': 'menu.inbounds',
  '/clients': 'menu.clients',
  '/groups': 'menu.groups',
  '/nodes': 'menu.nodes',
  '/settings': 'menu.settings',
  '/xray': 'menu.xray',
  '/api-docs': 'menu.apiDocs',
};

export function usePageTitle() {
  const { pathname } = useLocation();
  const { t } = useTranslation();

  useEffect(() => {
    const key = TITLE_KEYS[pathname];
    const title = key ? t(key) : '3X-UI';
    const host = window.location.hostname;
    document.title = host ? `${host} - ${title}` : title;
  }, [pathname, t]);
}
