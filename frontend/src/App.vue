<script setup>
import { ref, computed } from 'vue';
import { SizeFormatter, RandomUtil, Wireguard } from '@/utils';
import { useMediaQuery } from '@/composables/useMediaQuery.js';

const message = ref('Vue 3 + Ant Design Vue 4 scaffold is alive');
const count = ref(0);

const { isMobile } = useMediaQuery();

const fakeBytes = ref(1234567890);
const formatted = computed(() => SizeFormatter.sizeFormat(fakeBytes.value));
const uuid = ref(RandomUtil.randomUUID());
const keypair = ref(Wireguard.generateKeypair());
</script>

<template>
  <a-layout class="layout">
    <a-layout-header class="header">
      <h1>3x-ui (vue3-migration scaffold)</h1>
      <a-tag color="blue">isMobile: {{ isMobile }}</a-tag>
    </a-layout-header>
    <a-layout-content class="content">
      <a-space direction="vertical" :size="16" style="width: 100%">
        <a-alert :message="message" type="success" show-icon />
        <a-card title="Smoke test — toolchain">
          <a-space>
            <a-button type="primary" @click="count++">Clicked {{ count }} times</a-button>
            <a-button @click="count = 0">Reset</a-button>
          </a-space>
        </a-card>
        <a-card title="Smoke test — utility imports">
          <p><strong>SizeFormatter:</strong> {{ formatted }}</p>
          <p><strong>RandomUtil.randomUUID:</strong> <code>{{ uuid }}</code></p>
          <p><strong>Wireguard public key:</strong> <code>{{ keypair.publicKey }}</code></p>
          <a-button @click="uuid = RandomUtil.randomUUID()">Regenerate UUID</a-button>
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
  gap: 16px;
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
code {
  background: rgba(0, 0, 0, 0.06);
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 12px;
}
</style>
