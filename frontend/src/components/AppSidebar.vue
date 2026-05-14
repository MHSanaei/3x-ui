<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  DashboardOutlined,
  UserOutlined,
  SettingOutlined,
  ToolOutlined,
  ClusterOutlined,
  LogoutOutlined,
  CloseOutlined,
  MenuOutlined,
  ApiOutlined,
} from '@ant-design/icons-vue';

import { theme, currentTheme, toggleTheme, toggleUltra, pauseAnimationsUntilLeave } from '@/composables/useTheme.js';
import { HttpUtil } from '@/utils';

const { t } = useI18n();

const SIDEBAR_COLLAPSED_KEY = 'isSidebarCollapsed';

const props = defineProps({
  basePath: { type: String, default: '' },
  // Current request URI so the matching menu item highlights.
  requestUri: { type: String, default: '' },
});


const iconByName = {
  dashboard: DashboardOutlined,
  user: UserOutlined,
  setting: SettingOutlined,
  tool: ToolOutlined,
  cluster: ClusterOutlined,
  logout: LogoutOutlined,
  apidocs: ApiOutlined,
};

const prefix = props.basePath?.startsWith('/') ? props.basePath : `/${props.basePath || ''}`;

const tabs = computed(() => [
  { key: `${prefix}panel/`, icon: 'dashboard', title: t('menu.dashboard') },
  { key: `${prefix}panel/inbounds`, icon: 'user', title: t('menu.inbounds') },
  { key: `${prefix}panel/nodes`, icon: 'cluster', title: t('menu.nodes') },
  { key: `${prefix}panel/settings`, icon: 'setting', title: t('menu.settings') },
  { key: `${prefix}panel/xray`, icon: 'tool', title: t('menu.xray') },
  { key: `${prefix}panel/api-docs`, icon: 'apidocs', title: t('menu.apiDocs') },
  { key: 'logout', icon: 'logout', title: t('logout') },
]);

const navTabs = computed(() => tabs.value.filter((tab) => tab.icon !== 'logout'));
const utilTabs = computed(() => tabs.value.filter((tab) => tab.icon === 'logout'));
const activeTab = ref([props.requestUri]);
const drawerOpen = ref(false);
const collapsed = ref(JSON.parse(localStorage.getItem(SIDEBAR_COLLAPSED_KEY) || 'false'));
const drawerWidth = 'min(82vw, 320px)';

async function openLink(key) {
  if (key === 'logout') {
    await HttpUtil.post('/logout');
    window.location.href = props.basePath || '/';
    return;
  }
  if (key.startsWith('http')) {
    window.open(key);
  } else {
    window.location.href = key;
  }
}

function onCollapse(isCollapsed, type) {
  // Only persist explicit toggle clicks, not breakpoint-triggered collapses.
  if (type === 'clickTrigger') {
    localStorage.setItem(SIDEBAR_COLLAPSED_KEY, isCollapsed);
    collapsed.value = isCollapsed;
  }
}

function toggleDrawer() {
  drawerOpen.value = !drawerOpen.value;
}

function closeDrawer() {
  drawerOpen.value = false;
}

function cycleTheme() {
  pauseAnimationsUntilLeave('theme-cycle');
  if (!theme.isDark) {
    toggleTheme();
    if (theme.isUltra) toggleUltra();
  } else if (!theme.isUltra) {
    toggleUltra();
  } else {
    toggleUltra();
    toggleTheme();
  }
}
</script>

<template>
  <div class="ant-sidebar">
    <a-layout-sider :theme="currentTheme" collapsible :collapsed="collapsed" breakpoint="md" @collapse="onCollapse">
      <div class="sider-brand" :class="{ 'sider-brand-collapsed': collapsed }">
        <span class="brand-text">{{ collapsed ? '3X' : '3X-UI' }}</span>
        <button v-if="!collapsed" id="theme-cycle" type="button" class="theme-cycle" :aria-label="t('menu.theme')"
          :title="t('menu.theme')" @click="cycleTheme">
          <svg v-if="!theme.isDark" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
            stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="4" />
            <path
              d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
          </svg>
          <svg v-else-if="!theme.isUltra" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
            stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
          </svg>
          <svg v-else viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.5"
            stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
            <path fill="none" d="M19 3l0.7 1.4 1.4 0.7-1.4 0.7L19 7.2l-0.7-1.4-1.4-0.7 1.4-0.7z" />
          </svg>
        </button>
      </div>
      <a-menu :theme="currentTheme" mode="inline" :selected-keys="activeTab" class="sider-nav"
        @click="({ key }) => openLink(key)">
        <a-menu-item v-for="tab in navTabs" :key="tab.key">
          <component :is="iconByName[tab.icon]" />
          <span>{{ tab.title }}</span>
        </a-menu-item>
      </a-menu>
      <a-menu :theme="currentTheme" mode="inline" :selected-keys="activeTab" class="sider-utility"
        @click="({ key }) => openLink(key)">
        <a-menu-item v-for="tab in utilTabs" :key="tab.key">
          <component :is="iconByName[tab.icon]" />
          <span>{{ tab.title }}</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>

    <a-drawer placement="left" :closable="false" :open="drawerOpen" :wrap-class-name="currentTheme"
      :wrap-style="{ padding: 0 }" :width="drawerWidth"
      :body-style="{ padding: 0, display: 'flex', flexDirection: 'column', height: '100%' }"
      :header-style="{ display: 'none' }" @close="closeDrawer">
      <div class="drawer-header">
        <span class="drawer-brand">3X-UI</span>
        <div class="drawer-header-actions">
          <button id="theme-cycle-drawer" type="button" class="theme-cycle" :aria-label="t('menu.theme')"
            :title="t('menu.theme')" @click="cycleTheme">
            <svg v-if="!theme.isDark" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <circle cx="12" cy="12" r="4" />
              <path
                d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
            </svg>
            <svg v-else-if="!theme.isUltra" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.5"
              stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              <path fill="none" d="M19 3l0.7 1.4 1.4 0.7-1.4 0.7L19 7.2l-0.7-1.4-1.4-0.7 1.4-0.7z" />
            </svg>
          </button>
          <button class="drawer-close" type="button" :aria-label="t('close')" @click="closeDrawer">
            <CloseOutlined />
          </button>
        </div>
      </div>
      <a-menu :theme="currentTheme" mode="inline" :selected-keys="activeTab" class="drawer-menu drawer-nav"
        @click="({ key }) => openLink(key)">
        <a-menu-item v-for="tab in navTabs" :key="tab.key">
          <component :is="iconByName[tab.icon]" />
          <span>{{ tab.title }}</span>
        </a-menu-item>
      </a-menu>
      <a-menu :theme="currentTheme" mode="inline" :selected-keys="activeTab" class="drawer-menu drawer-utility"
        @click="({ key }) => openLink(key)">
        <a-menu-item v-for="tab in utilTabs" :key="tab.key">
          <component :is="iconByName[tab.icon]" />
          <span>{{ tab.title }}</span>
        </a-menu-item>
      </a-menu>
    </a-drawer>

    <button v-show="!drawerOpen" class="drawer-handle" type="button" :aria-label="t('menu.dashboard')"
      @click="toggleDrawer">
      <MenuOutlined />
    </button>
  </div>
</template>

<style scoped>
.ant-sidebar>.ant-layout-sider {
  position: sticky;
  top: 0;
  height: 100vh;
  align-self: flex-start;
}

.sider-brand,
.drawer-brand {
  font-weight: 600;
  font-size: 18px;
  letter-spacing: 0.5px;
  color: rgba(0, 0, 0, 0.88);
}

.sider-brand {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 14px 14px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
  user-select: none;
}

/* Collapsed sider only has room for the '3X' brand — center it and
 * hide the theme cycle button (which is `v-if`-ed out in template). */
.sider-brand-collapsed {
  justify-content: center;
  font-size: 16px;
  padding: 14px 4px;
  letter-spacing: 0;
}

.brand-text {
  flex: 1 1 auto;
}

.sider-brand-collapsed .brand-text {
  flex: 0 0 auto;
}

.theme-cycle {
  background: transparent;
  border: none;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: rgba(0, 0, 0, 0.75);
  padding: 0;
  flex-shrink: 0;
  transition: background-color 0.2s, transform 0.15s, color 0.2s;
}

.theme-cycle:hover,
.theme-cycle:focus-visible {
  background-color: rgba(64, 150, 255, 0.1);
  color: #4096ff;
  transform: scale(1.08);
  outline: none;
}

.theme-cycle svg {
  width: 16px;
  height: 16px;
}

.drawer-header-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.drawer-handle {
  position: fixed;
  top: 12px;
  left: 12px;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  border: none;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  cursor: pointer;
  display: none;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
}

.drawer-close {
  background: transparent;
  border: none;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 16px;
  color: rgba(0, 0, 0, 0.65);
}

.drawer-close:hover,
.drawer-close:focus-visible {
  background: rgba(128, 128, 128, 0.18);
}

.drawer-menu :deep(.ant-menu-item) {
  height: 48px;
  line-height: 48px;
  margin: 0;
  border-radius: 0;
}

.drawer-menu :deep(.ant-menu-item .anticon) {
  font-size: 16px;
}

.drawer-utility {
  margin-top: auto;
  border-top: 1px solid rgba(128, 128, 128, 0.15);
}

.ant-sidebar>.ant-layout-sider :deep(.ant-layout-sider-children) {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.sider-brand {
  flex: 0 0 auto;
}

.sider-nav {
  flex: 1 1 auto;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
}

.sider-utility {
  flex: 0 0 auto;
  border-top: 1px solid rgba(128, 128, 128, 0.15);
}

@media (max-width: 768px) {
  .drawer-handle {
    display: inline-flex;
  }

  .ant-sidebar>.ant-layout-sider :deep(.ant-layout-sider-children),
  .ant-sidebar>.ant-layout-sider :deep(.ant-layout-sider-trigger) {
    display: none;
  }

  .ant-sidebar>.ant-layout-sider {
    flex: 0 0 0 !important;
    max-width: 0 !important;
    min-width: 0 !important;
    width: 0 !important;
  }
}
</style>

<style>
body.dark .drawer-brand,
body.dark .sider-brand {
  color: rgba(255, 255, 255, 0.92);
}

html[data-theme='ultra-dark'] .drawer-brand,
html[data-theme='ultra-dark'] .sider-brand {
  color: #ffffff;
}

body.dark .drawer-close {
  color: rgba(255, 255, 255, 0.75);
}

html[data-theme='ultra-dark'] .drawer-close {
  color: rgba(255, 255, 255, 0.85);
}

body.dark .theme-cycle {
  color: rgba(255, 255, 255, 0.85);
}

html[data-theme='ultra-dark'] .theme-cycle {
  color: rgba(255, 255, 255, 0.92);
}

body.dark .ant-drawer .ant-drawer-content,
body.dark .ant-drawer .ant-drawer-body {
  background: #252526 !important;
}

html[data-theme='ultra-dark'] .ant-drawer .ant-drawer-content,
html[data-theme='ultra-dark'] .ant-drawer .ant-drawer-body {
  background: #0a0a0a !important;
}

.sider-nav .ant-menu-item-selected,
.sider-utility .ant-menu-item-selected,
.drawer-menu .ant-menu-item-selected {
  background-color: rgba(64, 150, 255, 0.2) !important;
  color: #4096ff !important;
}

.sider-nav .ant-menu-item-active:not(.ant-menu-item-selected),
.sider-utility .ant-menu-item-active:not(.ant-menu-item-selected),
.drawer-menu .ant-menu-item-active:not(.ant-menu-item-selected),
.sider-nav .ant-menu-item:not(.ant-menu-item-selected):not(.ant-menu-item-disabled):hover,
.sider-utility .ant-menu-item:not(.ant-menu-item-selected):not(.ant-menu-item-disabled):hover,
.drawer-menu .ant-menu-item:not(.ant-menu-item-selected):not(.ant-menu-item-disabled):hover {
  background-color: rgba(64, 150, 255, 0.1) !important;
  color: #4096ff !important;
}
</style>
