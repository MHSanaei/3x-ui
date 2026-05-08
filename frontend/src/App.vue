<script setup>
import { ref } from 'vue';

const message = ref('Vue 3 + Ant Design Vue 4 scaffold is alive');
const count = ref(0);
</script>

<template>
  <a-layout class="layout">
    <a-layout-header class="header">
      <h1>3x-ui (vue3-migration scaffold)</h1>
    </a-layout-header>
    <a-layout-content class="content">
      <a-space direction="vertical" :size="16" style="width: 100%">
        <a-alert :message="message" type="success" show-icon />
        <a-card title="Smoke test">
          <p>If you see this card with a styled button below, the toolchain works.</p>
          <a-space>
            <a-button type="primary" @click="count++">Clicked {{ count }} times</a-button>
            <a-button @click="count = 0">Reset</a-button>
          </a-space>
        </a-card>
      </a-space>
    </a-layout-content>
  </a-layout>
</template>

<style>
.layout {
  min-height: 100vh;
}
.header {
  background: #001529;
  color: #fff;
  display: flex;
  align-items: center;
  padding: 0 24px;
}
.header h1 {
  color: #fff;
  margin: 0;
  font-size: 18px;
}
.content {
  padding: 24px;
  background: #f0f2f5;
}
</style>
