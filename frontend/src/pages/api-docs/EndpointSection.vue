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
  icon: { type: Object, default: null },
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
        <component v-if="icon" :is="icon" class="section-icon" />
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
  border: 1px solid rgba(128, 128, 128, 0.12);
  border-radius: 8px;
  padding: 20px 24px;
  margin-bottom: 16px;
  transition: box-shadow 0.2s, border-color 0.2s;
}

.api-section:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
}

.section-header:hover .collapse-icon,
.section-header:hover .section-icon {
  color: #1677ff;
}

.section-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.collapse-icon {
  font-size: 12px;
  color: rgba(0, 0, 0, 0.4);
  transition: color 0.2s;
}

.section-icon {
  font-size: 18px;
  color: rgba(0, 0, 0, 0.45);
  transition: color 0.2s;
}

.section-title {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
  color: rgba(0, 0, 0, 0.88);
}

.endpoint-count {
  font-size: 11px;
  font-weight: 600;
  color: rgba(0, 0, 0, 0.45);
  white-space: nowrap;
  background: rgba(128, 128, 128, 0.08);
  padding: 3px 10px;
  border-radius: 12px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.section-description {
  margin: 12px 0 14px;
  color: rgba(0, 0, 0, 0.65);
  line-height: 1.6;
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
  border-color: rgba(255, 255, 255, 0.08);
}

body.dark .api-section:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.25);
}

html[data-theme='ultra-dark'] .api-section {
  background: #0a0a0a;
  border-color: rgba(255, 255, 255, 0.06);
}

html[data-theme='ultra-dark'] .api-section:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.4);
}

body.dark .section-title {
  color: rgba(255, 255, 255, 0.92);
}

body.dark .section-icon {
  color: rgba(255, 255, 255, 0.5);
}

body.dark .section-description {
  color: rgba(255, 255, 255, 0.7);
}

body.dark .block-label {
  color: rgba(255, 255, 255, 0.55);
}

body.dark .endpoint-count {
  color: rgba(255, 255, 255, 0.55);
  background: rgba(255, 255, 255, 0.06);
}
</style>
