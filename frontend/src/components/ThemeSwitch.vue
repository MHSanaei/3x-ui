<script setup>
import { computed } from 'vue';
import { BulbFilled, BulbOutlined } from '@ant-design/icons-vue';
import { theme, currentTheme, toggleTheme, toggleUltra, pauseAnimationsUntilLeave } from '@/composables/useTheme.js';

const BulbIcon = computed(() => (theme.isDark ? BulbFilled : BulbOutlined));

function onDarkChange() {
  pauseAnimationsUntilLeave('change-theme');
  toggleTheme();
}

function onUltraClick() {
  pauseAnimationsUntilLeave('change-theme-ultra');
  toggleUltra();
}
</script>

<template>
  <a-menu :theme="currentTheme" mode="inline" :selected-keys="[]">
    <a-sub-menu>
      <template #title>
        <span>
          <component :is="BulbIcon" />
          <span class="theme-label">Theme</span>
        </span>
      </template>

      <a-menu-item id="change-theme" class="ant-menu-theme-switch">
        <span>Dark</span>
        <a-switch :style="{ marginLeft: '2px' }" size="small" :checked="theme.isDark" @change="onDarkChange" />
      </a-menu-item>

      <a-menu-item v-if="theme.isDark" id="change-theme-ultra" class="ant-menu-theme-switch">
        <span>Ultra dark</span>
        <a-checkbox :style="{ marginLeft: '2px' }" :checked="theme.isUltra" @click="onUltraClick" />
      </a-menu-item>
    </a-sub-menu>
  </a-menu>
</template>

<style scoped>
.theme-label {
  margin-left: 8px;
}
</style>
