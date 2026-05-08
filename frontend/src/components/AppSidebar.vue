<script setup>
import { ref } from 'vue';
import {
  DashboardOutlined,
  UserOutlined,
  SettingOutlined,
  ToolOutlined,
  LogoutOutlined,
  CloseOutlined,
  MenuFoldOutlined,
} from '@ant-design/icons-vue';

import { currentTheme } from '@/composables/useTheme.js';
import ThemeSwitch from '@/components/ThemeSwitch.vue';

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
  logout: LogoutOutlined,
};

const tabs = [
  { key: `${props.basePath}panel/`,         icon: 'dashboard', title: 'Dashboard' },
  { key: `${props.basePath}panel/inbounds`, icon: 'user',      title: 'Inbounds' },
  { key: `${props.basePath}panel/settings`, icon: 'setting',   title: 'Settings' },
  { key: `${props.basePath}panel/xray`,     icon: 'tool',      title: 'Xray' },
  { key: `${props.basePath}logout/`,        icon: 'logout',    title: 'Logout' },
];

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
    <a-layout-sider
      :theme="currentTheme"
      collapsible
      :collapsed="collapsed"
      breakpoint="md"
      @collapse="onCollapse"
    >
      <ThemeSwitch />
      <a-menu
        :theme="currentTheme"
        mode="inline"
        :selected-keys="activeTab"
        @click="({ key }) => openLink(key)"
      >
        <a-menu-item v-for="tab in tabs" :key="tab.key">
          <component :is="iconByName[tab.icon]" />
          <span>{{ tab.title }}</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>

    <a-drawer
      placement="left"
      :closable="false"
      :open="drawerOpen"
      :wrap-class-name="currentTheme"
      :wrap-style="{ padding: 0 }"
      :style="{ height: '100%' }"
      @close="closeDrawer"
    >
      <ThemeSwitch />
      <a-menu
        :theme="currentTheme"
        mode="inline"
        :selected-keys="activeTab"
        @click="({ key }) => openLink(key)"
      >
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
.ant-sidebar > .ant-layout-sider {
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
}
</style>
