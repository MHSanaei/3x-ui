<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  EditOutlined,
  DeleteOutlined,
  PlusOutlined,
  ThunderboltOutlined,
  ExclamationCircleOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
  InfoCircleOutlined,
  MoreOutlined,
  RightOutlined,
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

const dataSource = computed(() =>
  props.nodes.map((n) => ({
    ...n,
    url: `${n.scheme}://${n.address}:${n.port}${n.basePath || '/'}`,
    key: n.id,
  })),
);

const showAddress = ref(false);

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

const statsNode = ref(null);
function openStats(node) {
  statsNode.value = node;
}
function closeStats() {
  statsNode.value = null;
}

const expandedIds = ref(new Set());
function toggleExpanded(id) {
  const next = new Set(expandedIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  expandedIds.value = next;
}
function isExpanded(id) {
  return expandedIds.value.has(id);
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

    <!-- ====================== Mobile: card list ======================= -->
    <div v-if="isMobile" class="node-cards">
      <div v-if="dataSource.length === 0" class="card-empty">—</div>

      <div v-for="record in dataSource" :key="record.id" class="node-card">
        <div class="card-head" @click="toggleExpanded(record.id)">
          <RightOutlined class="card-expand" :class="{ 'is-expanded': isExpanded(record.id) }" />
          <a-badge
            :status="statusColor(record.status) === 'green' ? 'success' : (statusColor(record.status) === 'red' ? 'error' : 'default')" />
          <span class="node-name">{{ record.name }}</span>
          <div class="card-actions" @click.stop>
            <a-tooltip :title="t('info')">
              <InfoCircleOutlined class="row-action-trigger" @click="openStats(record)" />
            </a-tooltip>
            <a-switch :checked="record.enable" size="small" @change="(v) => emit('toggle-enable', record, v)" />
            <a-dropdown :trigger="['click']" placement="bottomRight">
              <MoreOutlined class="row-action-trigger" @click.prevent />
              <template #overlay>
                <a-menu>
                  <a-menu-item key="probe" @click="emit('probe', record)">
                    <ThunderboltOutlined /> {{ t('pages.nodes.probe') }}
                  </a-menu-item>
                  <a-menu-item key="edit" @click="emit('edit', record)">
                    <EditOutlined /> {{ t('edit') }}
                  </a-menu-item>
                  <a-menu-item key="delete" class="danger-item" @click="emit('delete', record)">
                    <DeleteOutlined /> {{ t('delete') }}
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </div>
        </div>

        <div v-if="isExpanded(record.id)" class="card-history">
          <NodeHistoryPanel :node="record" />
        </div>
      </div>
    </div>

    <a-modal v-if="isMobile" :open="!!statsNode" :footer="null" :width="360" centered
      :title="statsNode ? statsNode.name : ''" @cancel="closeStats">
      <div v-if="statsNode" class="card-stats">
        <div v-if="statsNode.remark" class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.name') }}</span>
          <span>{{ statsNode.remark }}</span>
        </div>
        <div class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.address') }}</span>
          <a :href="statsNode.url" target="_blank" rel="noopener noreferrer"
            :class="showAddress ? 'address-visible' : 'address-hidden'">{{ statsNode.url }}</a>
          <a-tooltip :title="t('pages.index.toggleIpVisibility')">
            <component :is="showAddress ? EyeOutlined : EyeInvisibleOutlined" class="ip-toggle-icon"
              @click="showAddress = !showAddress" />
          </a-tooltip>
        </div>
        <div class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.status') }}</span>
          <a-badge
            :status="statusColor(statsNode.status) === 'green' ? 'success' : (statusColor(statsNode.status) === 'red' ? 'error' : 'default')" />
          <span>{{ t(`pages.nodes.statusValues.${statsNode.status || 'unknown'}`) }}</span>
          <a-tooltip v-if="statsNode.lastError" :title="statsNode.lastError">
            <ExclamationCircleOutlined style="color: #faad14" />
          </a-tooltip>
        </div>
        <div class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.cpu') }}</span>
          <a-tag>{{ formatPct(statsNode.cpuPct) }}</a-tag>
        </div>
        <div class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.mem') }}</span>
          <a-tag>{{ formatPct(statsNode.memPct) }}</a-tag>
        </div>
        <div class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.xrayVersion') }}</span>
          <a-tag>{{ statsNode.xrayVersion || '-' }}</a-tag>
        </div>
        <div class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.uptime') }}</span>
          <a-tag>{{ formatUptime(statsNode.uptimeSecs) }}</a-tag>
        </div>
        <div class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.latency') }}</span>
          <a-tag>
            <template v-if="statsNode.latencyMs > 0">{{ statsNode.latencyMs }} ms</template>
            <template v-else>-</template>
          </a-tag>
        </div>
        <div class="stat-row">
          <span class="stat-label">{{ t('pages.nodes.lastHeartbeat') }}</span>
          <a-tag>{{ relativeTime(statsNode.lastHeartbeat) }}</a-tag>
        </div>
      </div>
    </a-modal>

    <!-- ====================== Desktop: a-table ======================== -->
    <a-table v-else :data-source="dataSource" :pagination="false" :loading="loading" :scroll="{ x: 'max-content' }"
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

      <a-table-column data-index="url" :ellipsis="true">
        <template #title>
          <span class="address-header">
            {{ t('pages.nodes.address') }}
            <a-tooltip :title="t('pages.index.toggleIpVisibility')">
              <component :is="showAddress ? EyeOutlined : EyeInvisibleOutlined" class="ip-toggle-icon"
                @click="showAddress = !showAddress" />
            </a-tooltip>
          </span>
        </template>
        <template #default="{ record }">
          <a :href="record.url" target="_blank" rel="noopener noreferrer"
            :class="showAddress ? 'address-visible' : 'address-hidden'">{{ record.url }}</a>
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

.address-header {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.ip-toggle-icon {
  cursor: pointer;
  font-size: 14px;
  opacity: 0.7;
}

.ip-toggle-icon:hover {
  opacity: 1;
}

.address-hidden {
  filter: blur(5px);
  transition: filter 0.2s ease;
}

.address-visible {
  filter: none;
}

.node-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 4px;
}

.node-card {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 10px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.02);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

:global(body.dark) .node-card {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
}

.card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.card-expand {
  font-size: 12px;
  opacity: 0.6;
  transition: transform 150ms ease;
  flex-shrink: 0;
}

.card-expand.is-expanded {
  transform: rotate(90deg);
}

.node-name {
  font-weight: 600;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.row-action-trigger {
  font-size: 20px;
  cursor: pointer;
}

.card-stats {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.stat-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.stat-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  opacity: 0.6;
  min-width: 96px;
  flex-shrink: 0;
}

.card-stats :deep(.ant-tag) {
  margin: 0;
}

.card-history {
  margin-top: 4px;
  padding-top: 8px;
  border-top: 1px solid rgba(128, 128, 128, 0.15);
}

.card-empty {
  text-align: center;
  opacity: 0.4;
  padding: 20px 0;
}

.danger-item {
  color: #ff4d4f;
}
</style>
