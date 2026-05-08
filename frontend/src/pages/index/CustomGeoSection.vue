<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import {
  PlusOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  InboxOutlined,
} from '@ant-design/icons-vue';

import { HttpUtil, ClipboardManager } from '@/utils';
import CustomGeoFormModal from './CustomGeoFormModal.vue';

const { t } = useI18n();

const props = defineProps({
  // Re-fetch the list when the parent collapse expands this section.
  active: { type: Boolean, default: false },
});

const list = ref([]);
const loading = ref(false);
const updatingAll = ref(false);
const actionId = ref(null);

const formOpen = ref(false);
const editingRecord = ref(null);

// Computed so column titles re-render after a locale swap.
const columns = computed(() => [
  { title: t('pages.index.customGeoAlias'), key: 'alias', width: 200 },
  { title: t('pages.index.customGeoUrl'), key: 'url', ellipsis: true },
  { title: t('pages.index.customGeoExtColumn'), key: 'extDat', width: 220 },
  { title: t('pages.index.customGeoLastUpdated'), key: 'lastUpdatedAt', width: 140 },
  { title: t('pages.index.customGeoActions'), key: 'action', width: 120 },
]);

async function loadList() {
  loading.value = true;
  try {
    const msg = await HttpUtil.get('/panel/api/custom-geo/list');
    if (msg?.success && Array.isArray(msg.obj)) list.value = msg.obj;
  } finally {
    loading.value = false;
  }
}

function openAdd() {
  editingRecord.value = null;
  formOpen.value = true;
}

function openEdit(record) {
  editingRecord.value = record;
  formOpen.value = true;
}

function extDisplay(record) {
  const fn = record.type === 'geoip'
    ? `geoip_${record.alias}.dat`
    : `geosite_${record.alias}.dat`;
  return `ext:${fn}:tag`;
}

async function copyExt(record) {
  const text = extDisplay(record);
  const ok = await ClipboardManager.copyText(text);
  if (ok) message.success(`${t('copied')}: ${text}`);
}

function formatTime(ts) {
  if (!ts) return '';
  const d = new Date(ts * 1000);
  if (isNaN(d.getTime())) return String(ts);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// Tiny inline relative-time formatter so we don't pull in moment.
function relativeTime(ts) {
  if (!ts) return '';
  const diff = Math.floor(Date.now() / 1000) - ts;
  if (diff < 60) return 'just now';
  if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} h ago`;
  if (diff < 2592000) return `${Math.floor(diff / 86400)} d ago`;
  return formatTime(ts);
}

function confirmDelete(record) {
  Modal.confirm({
    title: t('pages.index.customGeoDelete'),
    content: t('pages.index.customGeoDeleteConfirm'),
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await HttpUtil.post(`/panel/api/custom-geo/delete/${record.id}`);
      if (msg?.success) await loadList();
    },
  });
}

async function downloadOne(id) {
  actionId.value = id;
  try {
    const msg = await HttpUtil.post(`/panel/api/custom-geo/download/${id}`);
    if (msg?.success) await loadList();
  } finally {
    actionId.value = null;
  }
}

async function updateAll() {
  updatingAll.value = true;
  try {
    const msg = await HttpUtil.post('/panel/api/custom-geo/update-all');
    const ok = msg?.obj?.succeeded?.length || 0;
    const failed = msg?.obj?.failed?.length || 0;
    if (msg?.success || ok > 0) {
      await loadList();
      if (failed > 0) message.warning(`Updated ${ok}, failed ${failed}`);
    }
  } finally {
    updatingAll.value = false;
  }
}

// Lazy-load: only fetch when the parent collapse opens this panel.
watch(() => props.active, (next) => { if (next) loadList(); }, { immediate: true });
</script>

<template>
  <div class="custom-geo-section">
    <a-alert type="info" show-icon class="mb-10" :message="t('pages.index.customGeoRoutingHint')" />

    <div class="toolbar">
      <a-button type="primary" :loading="loading" @click="openAdd">
        <template #icon>
          <PlusOutlined />
        </template>
        {{ t('pages.index.customGeoAdd') }}
      </a-button>
      <a-button :loading="updatingAll" :disabled="!list.length" @click="updateAll">
        <template #icon>
          <ReloadOutlined />
        </template>
        {{ t('pages.index.geofilesUpdateAll') }}
      </a-button>
      <span v-if="list.length" class="custom-geo-count">{{ list.length }}</span>
    </div>

    <a-table :columns="columns" :data-source="list" :pagination="false" :row-key="(r) => r.id" :loading="loading"
      size="small" :scroll="{ x: 760 }">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'alias'">
          <div class="custom-geo-alias-cell">
            <a-tag :color="record.type === 'geoip' ? 'cyan' : 'purple'" class="custom-geo-type-tag">
              {{ record.type }}
            </a-tag>
            <span class="custom-geo-alias">{{ record.alias }}</span>
          </div>
        </template>

        <template v-else-if="column.key === 'url'">
          <a-tooltip placement="topLeft" :title="record.url">
            <a :href="record.url" target="_blank" rel="noopener noreferrer" class="custom-geo-url">
              {{ record.url }}
            </a>
          </a-tooltip>
        </template>

        <template v-else-if="column.key === 'extDat'">
          <a-tooltip :title="t('copy')">
            <code class="custom-geo-ext-code custom-geo-copyable" @click="copyExt(record)">
              {{ extDisplay(record) }}
            </code>
          </a-tooltip>
        </template>

        <template v-else-if="column.key === 'lastUpdatedAt'">
          <a-tooltip v-if="record.lastUpdatedAt" :title="formatTime(record.lastUpdatedAt)">
            <span>{{ relativeTime(record.lastUpdatedAt) }}</span>
          </a-tooltip>
          <span v-else class="custom-geo-muted">—</span>
        </template>

        <template v-else-if="column.key === 'action'">
          <a-space size="small">
            <a-tooltip :title="t('pages.index.customGeoEdit')">
              <a-button type="link" size="small" @click="openEdit(record)">
                <template #icon>
                  <EditOutlined />
                </template>
              </a-button>
            </a-tooltip>
            <a-tooltip :title="t('pages.index.customGeoDownload')">
              <a-button type="link" size="small" :loading="actionId === record.id" @click="downloadOne(record.id)">
                <template #icon>
                  <ReloadOutlined />
                </template>
              </a-button>
            </a-tooltip>
            <a-tooltip :title="t('pages.index.customGeoDelete')">
              <a-button type="link" size="small" danger @click="confirmDelete(record)">
                <template #icon>
                  <DeleteOutlined />
                </template>
              </a-button>
            </a-tooltip>
          </a-space>
        </template>
      </template>

      <template #emptyText>
        <div class="custom-geo-empty">
          <InboxOutlined class="custom-geo-empty-icon" />
          <div>{{ t('pages.index.customGeoEmpty') }}</div>
        </div>
      </template>
    </a-table>

    <CustomGeoFormModal v-model:open="formOpen" :record="editingRecord" @saved="loadList" />
  </div>
</template>

<style scoped>
.mb-10 {
  margin-bottom: 10px;
}

.toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}

.custom-geo-count {
  margin-left: 4px;
  padding: 2px 8px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.05);
  font-size: 12px;
  opacity: 0.75;
}

:global(body.dark) .custom-geo-count {
  background: rgba(255, 255, 255, 0.08);
}

.custom-geo-alias-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.custom-geo-alias {
  font-weight: 500;
  word-break: break-all;
}

.custom-geo-type-tag {
  margin: 0;
}

.custom-geo-url {
  word-break: break-all;
}

.custom-geo-ext-code {
  cursor: pointer;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.05);
  user-select: all;
}

.custom-geo-copyable:hover {
  background: rgba(0, 0, 0, 0.1);
}

:global(body.dark) .custom-geo-ext-code {
  background: rgba(255, 255, 255, 0.08);
}

:global(body.dark) .custom-geo-copyable:hover {
  background: rgba(255, 255, 255, 0.14);
}

.custom-geo-muted {
  opacity: 0.5;
}

.custom-geo-empty {
  text-align: center;
  padding: 18px 0;
  opacity: 0.6;
}

.custom-geo-empty-icon {
  font-size: 32px;
  margin-bottom: 6px;
  display: block;
}
</style>
