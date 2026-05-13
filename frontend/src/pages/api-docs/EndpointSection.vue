<script setup>
import { computed } from 'vue';
import {
  DownOutlined,
  RightOutlined,
} from '@ant-design/icons-vue';
import EndpointRow from './EndpointRow.vue';
import { safeInlineHtml } from './endpoints.js';

const props = defineProps({
  section: { type: Object, required: true },
  collapsed: { type: Boolean, default: false },
});

const emit = defineEmits(['toggle']);

const endpointLabel = computed(() =>
  props.section.endpoints.length === 1
    ? '1 endpoint'
    : `${props.section.endpoints.length} endpoints`
);
</script>

<template>
  <section :id="section.id" class="api-section">
    <div class="section-header" @click="emit('toggle')">
      <div class="section-header-left">
        <DownOutlined v-if="!collapsed" class="collapse-icon" />
        <RightOutlined v-else class="collapse-icon" />
        <h2 class="section-title">{{ section.title }}</h2>
      </div>
      <span class="endpoint-count">{{ endpointLabel }}</span>
    </div>
    <p v-if="section.description && !collapsed" class="section-description" v-html="safeInlineHtml(section.description)"></p>

    <div v-if="section.subHeader && !collapsed" class="sub-header-block">
      <div class="block-label">Response headers</div>
      <a-table
        :columns="[{ title: 'Header', dataIndex: 'name', key: 'name', width: 240 }, { title: 'Description', dataIndex: 'desc', key: 'desc' }]"
        :data-source="section.subHeader"
        :pagination="false"
        size="small"
        row-key="name"
      >
        <template #bodyCell="{ column, text }">
          <span v-if="column.dataIndex === 'desc'" v-html="safeInlineHtml(text)"></span>
          <template v-else>{{ text }}</template>
        </template>
      </a-table>
    </div>

    <div v-show="!collapsed" class="endpoints">
      <EndpointRow v-for="(endpoint, idx) in section.endpoints" :key="idx" :endpoint="endpoint" />
    </div>
  </section>
</template>

<style scoped>
.api-section {
  background: #fff;
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 8px;
  padding: 16px 24px;
  margin-bottom: 16px;
  scroll-margin-top: 16px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
}

.section-header:hover .collapse-icon {
  color: #1677ff;
}

.section-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.collapse-icon {
  font-size: 12px;
  color: rgba(0, 0, 0, 0.45);
  transition: color 0.2s;
}

.section-title {
  font-size: 20px;
  font-weight: 600;
  margin: 0;
  color: rgba(0, 0, 0, 0.88);
}

.endpoint-count {
  font-size: 12px;
  color: rgba(0, 0, 0, 0.45);
  white-space: nowrap;
}

.section-description {
  margin: 10px 0 14px;
  color: rgba(0, 0, 0, 0.65);
  line-height: 1.55;
}

.sub-header-block {
  margin-bottom: 14px;
}

.block-label {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: rgba(0, 0, 0, 0.5);
  margin-bottom: 6px;
}

.endpoints {
  padding-top: 8px;
  border-top: 1px solid rgba(128, 128, 128, 0.1);
}

.endpoints > :first-child {
  padding-top: 0;
}
</style>

<style>
body.dark .api-section {
  background: #252526;
  border-color: rgba(255, 255, 255, 0.1);
}

html[data-theme='ultra-dark'] .api-section {
  background: #0a0a0a;
  border-color: rgba(255, 255, 255, 0.08);
}

body.dark .section-title {
  color: rgba(255, 255, 255, 0.92);
}

body.dark .section-description {
  color: rgba(255, 255, 255, 0.7);
}

body.dark .block-label {
  color: rgba(255, 255, 255, 0.55);
}
</style>
