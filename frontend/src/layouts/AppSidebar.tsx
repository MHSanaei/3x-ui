import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ComponentType, CSSProperties } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Drawer, Layout, Menu } from 'antd';
import type { MenuProps } from 'antd';
import {
  ApiOutlined,
  ApartmentOutlined,
  CloseOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  CodeOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  DiscordOutlined,
  ExportOutlined,
  GlobalOutlined,
  ImportOutlined,
  LogoutOutlined,
  MailOutlined,
  MenuOutlined,
  MessageOutlined,
  PushpinFilled,
  PushpinOutlined,
  SafetyOutlined,
  SettingOutlined,
  SwapOutlined,
  TagsOutlined,
  TeamOutlined,
  ToolOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import { useTheme } from '@/hooks/useTheme';
import { useAllSettings } from '@/api/queries/useAllSettings';
import './AppSidebar.css';

const LOGOUT_KEY = '__logout__';
const RAIL_WIDTH = 72;
const SIDER_WIDTH = 220;
const SIDEBAR_PINNED_KEY = 'sidebar-pinned';

let hoveredAcrossRemounts = false;

type IconName =
  | 'dashboard'
  | 'inbound'
  | 'team'
  | 'groups'
  | 'setting'
  | 'tool'
  | 'cluster'
  | 'hosts'
  | 'logout'
  | 'apidocs'
  | 'outbound'
  | 'routing'
  | 'telemt';

const iconByName: Record<IconName, ComponentType> = {
  dashboard: DashboardOutlined,
  inbound: ImportOutlined,
  team: TeamOutlined,
  groups: TagsOutlined,
  setting: SettingOutlined,
  tool: ToolOutlined,
  cluster: ClusterOutlined,
  hosts: GlobalOutlined,
  logout: LogoutOutlined,
  apidocs: ApiOutlined,
  outbound: ExportOutlined,
  routing: SwapOutlined,
  telemt: MessageOutlined,
};

function readSidebarPinned() {
  try {
    return localStorage.getItem(SIDEBAR_PINNED_KEY) === 'true';
  } catch {
    return false;
  }
}
function saveSidebarPinned(pinned: boolean) {
  try {
    localStorage.setItem(SIDEBAR_PINNED_KEY, String(pinned));
  } catch {}
}

function AppSidebar() {
  const { t } = useTranslation();
  const { mode } = useTheme();
  const navigate = useNavigate();
  const { pathname, hash } = useLocation();
  const { allSetting } = useAllSettings();
  const showSubFormats = !!(allSetting.subJsonEnable || allSetting.subClashEnable);
  const showSubBalancers = !!allSetting.subJsonEnable;
  const [hovered, setHovered] = useState(() => hoveredAcrossRemounts);
  const [pinned, setPinned] = useState(readSidebarPinned);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const resetDrawerSideEffects = useCallback(() => {
    document.documentElement.style.removeProperty('overflow');
    document.body.style.removeProperty('overflow');
    document.body.style.removeProperty('touch-action');
  }, []);
  const closeDrawer = useCallback(() => {
    setDrawerOpen(false);
    resetDrawerSideEffects();
  }, [resetDrawerSideEffects]);
  const railCollapsed = !hovered && !pinned;
  const railStyle = useMemo(
    () => ({ '--sider-rail': `${pinned ? SIDER_WIDTH : RAIL_WIDTH}px` }) as CSSProperties,
    [pinned],
  );
  const rootRef = useRef<HTMLDivElement>(null);
  const updateHovered = useCallback((value: boolean) => {
    hoveredAcrossRemounts = value;
    setHovered(value);
  }, []);
  const togglePinned = useCallback(() => {
    const next = !pinned;
    saveSidebarPinned(next);
    setPinned(next);
  }, [pinned]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const el = rootRef.current;
      if (el) updateHovered(el.matches(':hover'));
    }, 150);
    return () => window.clearTimeout(timer);
  }, [updateHovered]);

  const currentTheme: 'light' | 'dark' = mode === 'light' || mode === 'colorful' ? 'light' : 'dark';
  const tabs = useMemo<{ key: string; icon: IconName; title: string }[]>(
    () => [
      { key: '/', icon: 'dashboard', title: t('menu.dashboard') },
      { key: '/inbounds', icon: 'inbound', title: t('menu.inbounds') },
      { key: '/clients', icon: 'team', title: t('menu.clients') },
      { key: '/groups', icon: 'groups', title: t('menu.groups') },
      { key: '/nodes', icon: 'cluster', title: t('menu.nodes') },
      { key: '/hosts', icon: 'hosts', title: t('menu.hosts') },
      { key: '/outbound', icon: 'outbound', title: t('menu.outbounds') },
      { key: '/routing', icon: 'routing', title: t('menu.routing') },
      { key: '/telemt', icon: 'telemt', title: t('menu.telemt') },
      { key: '/settings', icon: 'setting', title: t('menu.settings') },
      { key: '/xray', icon: 'tool', title: t('menu.xray') },
      { key: '/api-docs', icon: 'apidocs', title: t('menu.apiDocs') },
      { key: LOGOUT_KEY, icon: 'logout', title: t('logout') },
    ],
    [t],
  );
  const navItems = useMemo(() => tabs.filter((tab) => tab.icon !== 'logout'), [tabs]);
  const utilItems = useMemo(() => tabs.filter((tab) => tab.icon === 'logout'), [tabs]);

  const settingsChildren = useMemo<NonNullable<MenuProps['items']>>(() => {
    const children: NonNullable<MenuProps['items']> = [
      {
        key: '/settings#general',
        icon: <SettingOutlined />,
        label: t('pages.settings.panelSettings'),
      },
      {
        key: '/settings#security',
        icon: <SafetyOutlined />,
        label: t('pages.settings.securitySettings'),
      },
      {
        key: '/settings#telegram',
        icon: <MessageOutlined />,
        label: t('pages.settings.TGBotSettings'),
      },
      { key: '/settings#email', icon: <MailOutlined />, label: t('pages.settings.emailSettings') },
      {
        key: '/settings#discord',
        icon: <DiscordOutlined />,
        label: t('pages.settings.discordSettings'),
      },
      {
        key: '/settings#subscription',
        icon: <CloudServerOutlined />,
        label: t('pages.settings.subSettings'),
      },
    ];
    if (showSubFormats)
      children.push({
        key: '/settings#subscription-formats',
        icon: <CodeOutlined />,
        label: t('menu.subFormats'),
      });
    if (showSubBalancers)
      children.push({
        key: '/settings#subscription-balancers',
        icon: <ApartmentOutlined />,
        label: t('pages.settings.subBalancers.menu'),
      });
    return children;
  }, [t, showSubFormats, showSubBalancers]);
  const xrayChildren = useMemo<NonNullable<MenuProps['items']>>(
    () => [
      { key: '/xray#basic', icon: <SettingOutlined />, label: t('pages.xray.basicTemplate') },
      { key: '/xray#balancer', icon: <ClusterOutlined />, label: t('pages.xray.Balancers') },
      { key: '/xray#dns', icon: <DatabaseOutlined />, label: 'DNS' },
      { key: '/xray#advanced', icon: <CodeOutlined />, label: t('pages.xray.advancedTemplate') },
    ],
    [t],
  );
  const settingsActive = pathname === '/settings';
  const xrayActive = pathname === '/xray';
  const selectedKey = settingsActive
    ? `/settings${hash || '#general'}`
    : xrayActive
      ? `/xray${hash || '#basic'}`
      : pathname === ''
        ? '/'
        : pathname;
  const openSubmenu = settingsActive ? '/settings' : xrayActive ? '/xray' : null;
  const [openKeys, setOpenKeys] = useState<string[]>(() => (openSubmenu ? [openSubmenu] : []));
  const visibleOpenKeys = useMemo(
    () => (!openSubmenu || openKeys.includes(openSubmenu) ? openKeys : [...openKeys, openSubmenu]),
    [openKeys, openSubmenu],
  );
  const toMenuItems = useCallback(
    (items: typeof tabs): MenuProps['items'] =>
      items.map((tab) => {
        const Icon = iconByName[tab.icon];
        if (tab.key === '/settings')
          return { key: tab.key, icon: <Icon />, label: tab.title, children: settingsChildren };
        if (tab.key === '/xray')
          return { key: tab.key, icon: <Icon />, label: tab.title, children: xrayChildren };
        return { key: tab.key, icon: <Icon />, label: tab.title, title: '' };
      }),
    [settingsChildren, xrayChildren],
  );
  const openLink = useCallback(
    async (key: string) => {
      if (key === LOGOUT_KEY) {
        await HttpUtil.post('/logout');
        window.location.href = window.X_UI_BASE_PATH || '/';
        return;
      }
      navigate(key);
    },
    [navigate],
  );
  const onMenuClick = useCallback<NonNullable<MenuProps['onClick']>>(
    ({ key }) => {
      void openLink(String(key));
    },
    [openLink],
  );

  return (
    <div
      ref={rootRef}
      className={`ant-sidebar${pinned ? ' sidebar-pinned' : ''}`}
      style={railStyle}
      onMouseEnter={() => updateHovered(true)}
      onMouseLeave={() => updateHovered(false)}
    >
      <Layout.Sider
        theme={currentTheme}
        width={SIDER_WIDTH}
        collapsedWidth={RAIL_WIDTH}
        collapsed={railCollapsed}
      >
        <div className="sider-brand">
          <div className="brand-block">
            <span className="brand-text">{railCollapsed ? '3X' : '3X-UI'}</span>
          </div>
          <div className="brand-actions">
            {!railCollapsed && (
              <button
                type="button"
                className="sidebar-pin"
                aria-label={t('menu.pinSidebar')}
                aria-pressed={pinned}
                title={t(pinned ? 'menu.unpinSidebar' : 'menu.pinSidebar')}
                onClick={togglePinned}
              >
                {pinned ? <PushpinFilled /> : <PushpinOutlined />}
              </button>
            )}
          </div>
        </div>
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          openKeys={railCollapsed ? undefined : visibleOpenKeys}
          onOpenChange={(keys) => setOpenKeys(keys as string[])}
          className="sider-nav"
          items={toMenuItems(navItems)}
          onClick={onMenuClick}
        />
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          className="sider-utility"
          items={toMenuItems(utilItems)}
          onClick={onMenuClick}
        />
      </Layout.Sider>
      <Drawer
        placement="left"
        closable={false}
        open={drawerOpen}
        rootClassName={currentTheme}
        size="min(82vw, 320px)"
        mask={{ enabled: true, blur: false }}
        styles={{
          wrapper: { padding: 0 },
          body: { padding: 0, display: 'flex', flexDirection: 'column', height: '100%' },
          header: { display: 'none' },
          mask: {
            backdropFilter: 'none',
            WebkitBackdropFilter: 'none',
            filter: 'none',
          },
        }}
        afterOpenChange={(open) => {
          if (!open) resetDrawerSideEffects();
        }}
        onClose={closeDrawer}
      >
        <div className="drawer-header">
          <div className="brand-block">
            <span className="drawer-brand">3X-UI</span>
          </div>
          <div className="drawer-header-actions">
            <button
              className="drawer-close"
              type="button"
              aria-label={t('close')}
              onClick={closeDrawer}
            >
              <CloseOutlined />
            </button>
          </div>
        </div>
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          openKeys={visibleOpenKeys}
          onOpenChange={(keys) => setOpenKeys(keys as string[])}
          className="drawer-menu drawer-nav"
          items={toMenuItems(navItems)}
          onClick={(info) => {
            onMenuClick(info);
            closeDrawer();
          }}
        />
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          className="drawer-menu drawer-utility"
          items={toMenuItems(utilItems)}
          onClick={(info) => {
            onMenuClick(info);
            closeDrawer();
          }}
        />
      </Drawer>
      {!drawerOpen && (
        <button
          className="drawer-handle"
          type="button"
          aria-label={t('menu.openMenu')}
          onClick={() => setDrawerOpen(true)}
        >
          <MenuOutlined />
        </button>
      )}
    </div>
  );
}

export default memo(AppSidebar);
