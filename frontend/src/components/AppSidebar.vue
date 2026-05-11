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
  MenuFoldOutlined,
} from '@ant-design/icons-vue';

import { currentTheme } from '@/composables/useTheme.js';
import ThemeSwitch from '@/components/ThemeSwitch.vue';

const { t } = useI18n();

const SIDEBAR_COLLAPSED_KEY = 'isSidebarCollapsed';

const props = defineProps({
  // Path prefix (e.g. /custom-base/) the panel is served under. Defaults
  // to '' which means tab keys end up as '/panel/...'. Pages pass the
  // value the Go backend gave them (in production via a meta tag).
  basePath: { type: String, default: '' },
  // Current request URI so the matching menu item highlights.
  requestUri: { type: String, default: '' },
});

// AD-Vue 4 dropped <a-icon :type="x"> in favor of explicit icon
// imports — keep a small name-to-component map so tab definitions stay
// declarative.
const iconByName = {
  dashboard: DashboardOutlined,
  user: UserOutlined,
  setting: SettingOutlined,
  tool: ToolOutlined,
  cluster: ClusterOutlined,
  logout: LogoutOutlined,
};

// basePath comes from Go (`/` by default, `/myprefix/` when configured) so
// these concatenations land on absolute paths. In dev we synthesize the prop
// from a window global which can be empty — force a leading slash so the
// browser doesn't resolve the link relative to the current pathname (which
// would turn /panel/settings + 'panel/...' into /panel/panel/...).
const prefix = props.basePath?.startsWith('/') ? props.basePath : `/${props.basePath || ''}`;

// Labels are i18n-driven so the sidebar matches the locale picked
// in panel settings without a page reload of the sidebar component.
const tabs = computed(() => [
  { key: `${prefix}panel/`, icon: 'dashboard', title: t('menu.dashboard') },
  { key: `${prefix}panel/inbounds`, icon: 'user', title: t('menu.inbounds') },
  { key: `${prefix}panel/nodes`, icon: 'cluster', title: t('menu.nodes') },
  { key: `${prefix}panel/settings`, icon: 'setting', title: t('menu.settings') },
  { key: `${prefix}panel/xray`, icon: 'tool', title: t('menu.xray') },
  { key: `${prefix}logout`, icon: 'logout', title: t('logout') },
]);

const activeTab = ref([props.requestUri]);

const drawerOpen = ref(false);
const collapsed = ref(JSON.parse(localStorage.getItem(SIDEBAR_COLLAPSED_KEY) || 'false'));

function openLink(key) {
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
</script>

<template>
  <div class="ant-sidebar">
    <a-layout-sider :theme="currentTheme" collapsible :collapsed="collapsed" breakpoint="md" @collapse="onCollapse">
      <ThemeSwitch />
      <a-menu :theme="currentTheme" mode="inline" :selected-keys="activeTab" @click="({ key }) => openLink(key)">
        <a-menu-item v-for="tab in tabs" :key="tab.key">
          <component :is="iconByName[tab.icon]" />
          <span>{{ tab.title }}</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>

    <a-drawer placement="left" :closable="false" :open="drawerOpen" :wrap-class-name="currentTheme"
      :wrap-style="{ padding: 0 }" :style="{ height: '100%' }" @close="closeDrawer">
      <ThemeSwitch />
      <a-menu :theme="currentTheme" mode="inline" :selected-keys="activeTab" @click="({ key }) => openLink(key)">
        <a-menu-item v-for="tab in tabs" :key="tab.key">
          <component :is="iconByName[tab.icon]" />
          <span>{{ tab.title }}</span>
        </a-menu-item>
      </a-menu>
    </a-drawer>

    <button class="drawer-handle" type="button" @click="toggleDrawer">
      <CloseOutlined v-if="drawerOpen" />
      <MenuFoldOutlined v-else />
    </button>
  </div>
</template>

<style scoped>
.ant-sidebar>.ant-layout-sider {
  height: 100%;
}

.drawer-handle {
  position: fixed;
  top: 16px;
  left: 16px;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  border: none;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  cursor: pointer;
  display: none;
  align-items: center;
  justify-content: center;
}

@media (max-width: 768px) {
  .drawer-handle {
    display: inline-flex;
  }

  /* On mobile the drawer is the menu — hide the inline sider's content
   * + the collapse trigger so the sider stops taking layout space and
   * leaves no remnant button next to the page. */
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
