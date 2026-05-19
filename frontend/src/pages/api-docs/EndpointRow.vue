<script setup>
import { computed } from 'vue';
import { methodColors, safeInlineHtml } from './endpoints.js';
import CodeBlock from './CodeBlock.vue';

const props = defineProps({
  endpoint: { type: Object, required: true },
});

const tagColor = computed(() => methodColors[props.endpoint.method] || 'default');
const hasParams = computed(() => Array.isArray(props.endpoint.params) && props.endpoint.params.length > 0);

const paramColumns = [
  { title: 'Name', dataIndex: 'name', key: 'name', width: 180 },
  { title: 'In', dataIndex: 'in', key: 'in', width: 100 },
  { title: 'Type', dataIndex: 'type', key: 'type', width: 120 },
  { title: 'Description', dataIndex: 'desc', key: 'desc' },
];
</script>

<template>
  <div class="endpoint-row">
    <div class="endpoint-header">
      <a-tag :color="tagColor" class="method-tag">{{ endpoint.method }}</a-tag>
      <code class="endpoint-path">{{ endpoint.path }}</code>
    </div>

    <p v-if="endpoint.summary" class="endpoint-summary" v-html="safeInlineHtml(endpoint.summary)"></p>

    <div v-if="hasParams" class="endpoint-block">
      <div class="block-label">Parameters</div>
      <a-table :columns="paramColumns" :data-source="endpoint.params" :pagination="false" size="small" row-key="name" />
    </div>

    <div v-if="endpoint.body" class="endpoint-block">
      <div class="block-label">Request body</div>
      <CodeBlock :code="endpoint.body" lang="json" />
    </div>

    <div v-if="endpoint.response" class="endpoint-block">
      <div class="block-label">Response</div>
      <CodeBlock :code="endpoint.response" lang="json" />
    </div>

    <div v-if="endpoint.errorResponse" class="endpoint-block">
      <div class="block-label error-label">Error response</div>
      <CodeBlock :code="endpoint.errorResponse" lang="json" />
    </div>
  </div>
</template>

<style scoped>
.endpoint-row {
  padding: 14px 0;
  margin: 0;
  transition: background 0.15s;
  border-radius: 6px;
  padding-left: 8px;
  padding-right: 8px;
  margin-left: -8px;
  margin-right: -8px;
}

.endpoint-row:hover {
  background: rgba(128, 128, 128, 0.03);
}

.endpoint-row + .endpoint-row {
  border-top: 1px solid rgba(128, 128, 128, 0.1);
}

.endpoint-header {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.method-tag {
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  letter-spacing: 0.5px;
  min-width: 56px;
  text-align: center;
  text-transform: uppercase;
  border-radius: 4px;
  padding: 2px 8px;
  line-height: 1.6;
}

.endpoint-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13.5px;
  word-break: break-all;
  color: rgba(0, 0, 0, 0.8);
  background: rgba(128, 128, 128, 0.06);
  padding: 2px 8px;
  border-radius: 4px;
}

.endpoint-summary {
  margin: 8px 0 0;
  color: rgba(0, 0, 0, 0.6);
  line-height: 1.6;
  font-size: 13.5px;
}

.endpoint-block {
  margin-top: 14px;
}

.block-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.6px;
  color: rgba(0, 0, 0, 0.45);
  margin-bottom: 6px;
}

.error-label {
  color: #cf222e;
}

.code-block {
  background: rgba(128, 128, 128, 0.08);
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 6px;
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
  line-height: 1.55;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-x: auto;
}
</style>

<style>
body.dark .endpoint-row:hover {
  background: rgba(255, 255, 255, 0.02);
}

body.dark .endpoint-row + .endpoint-row {
  border-top-color: rgba(255, 255, 255, 0.08);
}

body.dark .endpoint-path {
  color: rgba(255, 255, 255, 0.82);
  background: rgba(255, 255, 255, 0.05);
}

body.dark .endpoint-summary {
  color: rgba(255, 255, 255, 0.65);
}

body.dark .block-label {
  color: rgba(255, 255, 255, 0.45);
}

body.dark .error-label {
  color: #ff7b72;
}

body.dark .code-block {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.88);
}
</style>
