<script setup>
import { theme, toggleTheme, toggleUltra, pauseAnimationsUntilLeave } from '@/composables/useTheme.js';

function onDarkChange() {
  pauseAnimationsUntilLeave('change-theme');
  toggleTheme();
}

function onUltraClick() {
  toggleUltra();
}
</script>

<template>
  <a-space id="change-theme" direction="vertical" :size="10" :style="{ width: '100%' }">
    <a-space direction="horizontal" size="small">
      <a-switch size="small" :checked="theme.isDark" @change="onDarkChange" />
      <span>Dark</span>
    </a-space>
    <a-space v-if="theme.isDark" direction="horizontal" size="small">
      <a-checkbox :checked="theme.isUltra" @click="onUltraClick" />
      <span>Ultra dark</span>
    </a-space>
  </a-space>
</template>
