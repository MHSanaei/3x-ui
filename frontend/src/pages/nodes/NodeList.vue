<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  EditOutlined,
  DeleteOutlined,
  PlusOutlined,
  ThunderboltOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons-vue';
import NodeHistoryPanel from './NodeHistoryPanel.vue';

const props = defineProps({
  nodes: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  isMobile: { type: Boolean, default: false },
});

const emit = defineEmits([
  'add',
  'edit',
  'delete',
  'probe',
  'toggle-enable',
]);

const { t } = useI18n();

// Render the address column as a clickable URL so admins can jump to
// the remote panel directly from the list.
const dataSource = computed(() =>
  props.nodes.map((n) => ({
    ...n,
    url: `${n.scheme}://${n.address}:${n.port}${n.basePath || '/'}`,
    key: n.id,
  })),
);

function statusColor(status) {
  switch (status) {
    case 'online': return 'green';
    case 'offline': return 'red';
    default: return 'default';
  }
}

// Relative-time formatter — keeps the column compact and avoids
// pulling dayjs just for this single use.
function relativeTime(unixSeconds) {
  if (!unixSeconds) return t('pages.nodes.never');
  const diffSec = Math.max(0, Math.floor(Date.now() / 1000 - unixSeconds));
  if (diffSec < 5) return t('pages.nodes.justNow');
  if (diffSec < 60) return `${diffSec}s`;
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h`;
  return `${Math.floor(diffSec / 86400)}d`;
}

function formatUptime(secs) {
  if (!secs) return '-';
  const days = Math.floor(secs / 86400);
  const hours = Math.floor((secs % 86400) / 3600);
  if (days > 0) return `${days}d ${hours}h`;
  const mins = Math.floor((secs % 3600) / 60);
  if (hours > 0) return `${hours}h ${mins}m`;
  return `${mins}m`;
}

function formatPct(p) {
  if (typeof p !== 'number' || isNaN(p)) return '-';
  return `${p.toFixed(1)}%`;
}
</script>

<template>
  <a-card size="small" hoverable>
    <div class="toolbar">
      <a-button type="primary" @click="emit('add')">
        <template #icon>
          <PlusOutlined />
        </template>
        {{ t('pages.nodes.addNode') }}
      </a-button>
    </div>

    <a-table :data-source="dataSource" :pagination="false" :loading="loading" :scroll="{ x: 'max-content' }"
      size="middle" row-key="id">
      <template #expandedRowRender="{ record }">
        <NodeHistoryPanel :node="record" />
      </template>
      <a-table-column :title="t('pages.nodes.name')" data-index="name" :ellipsis="true">
        <template #default="{ record }">
          <div class="name-cell">
            <span class="name">{{ record.name }}</span>
            <span v-if="record.remark" class="remark">{{ record.remark }}</span>
          </div>
        </template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.address')" data-index="url" :ellipsis="true">
        <template #default="{ record }">
          <a :href="record.url" target="_blank" rel="noopener noreferrer">{{ record.url }}</a>
        </template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.status')" data-index="status" align="center">
        <template #default="{ record }">
          <a-space :size="4">
            <a-badge
              :status="statusColor(record.status) === 'green' ? 'success' : (statusColor(record.status) === 'red' ? 'error' : 'default')" />
            <span>{{ t(`pages.nodes.statusValues.${record.status || 'unknown'}`) }}</span>
            <a-tooltip v-if="record.lastError" :title="record.lastError">
              <ExclamationCircleOutlined style="color: #faad14" />
            </a-tooltip>
          </a-space>
        </template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.cpu')" data-index="cpuPct" align="center" :width="90">
        <template #default="{ record }">{{ formatPct(record.cpuPct) }}</template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.mem')" data-index="memPct" align="center" :width="90">
        <template #default="{ record }">{{ formatPct(record.memPct) }}</template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.xrayVersion')" data-index="xrayVersion" align="center">
        <template #default="{ record }">
          {{ record.xrayVersion || '-' }}
        </template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.uptime')" data-index="uptimeSecs" align="center">
        <template #default="{ record }">{{ formatUptime(record.uptimeSecs) }}</template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.latency')" data-index="latencyMs" align="center" :width="100">
        <template #default="{ record }">
          <span v-if="record.latencyMs > 0">{{ record.latencyMs }} ms</span>
          <span v-else>-</span>
        </template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.lastHeartbeat')" data-index="lastHeartbeat" align="center" :width="120">
        <template #default="{ record }">{{ relativeTime(record.lastHeartbeat) }}</template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.enable')" data-index="enable" align="center" :width="80">
        <template #default="{ record }">
          <a-switch :checked="record.enable" size="small" @change="(v) => emit('toggle-enable', record, v)" />
        </template>
      </a-table-column>

      <a-table-column :title="t('pages.nodes.actions')" align="center" :width="160" fixed="right">
        <template #default="{ record }">
          <a-space>
            <a-tooltip :title="t('pages.nodes.probe')">
              <a-button type="text" size="small" @click="emit('probe', record)">
                <template #icon>
                  <ThunderboltOutlined />
                </template>
              </a-button>
            </a-tooltip>
            <a-tooltip :title="t('edit')">
              <a-button type="text" size="small" @click="emit('edit', record)">
                <template #icon>
                  <EditOutlined />
                </template>
              </a-button>
            </a-tooltip>
            <a-tooltip :title="t('delete')">
              <a-button type="text" size="small" danger @click="emit('delete', record)">
                <template #icon>
                  <DeleteOutlined />
                </template>
              </a-button>
            </a-tooltip>
          </a-space>
        </template>
      </a-table-column>
    </a-table>
  </a-card>
</template>

<style scoped>
.toolbar {
  margin-bottom: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.name-cell {
  display: flex;
  flex-direction: column;
}

.name {
  font-weight: 500;
}

.remark {
  font-size: 12px;
  opacity: 0.65;
}
</style>
