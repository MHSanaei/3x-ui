<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PlusOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
} from '@ant-design/icons-vue';
import { Modal } from 'ant-design-vue';

import BalancerFormModal from './BalancerFormModal.vue';

const { t } = useI18n();

// Balancers tab — list + add/edit/delete over
// templateSettings.routing.balancers. The legacy panel kept the wire
// shape's `strategy: { type: 'random' }` nesting only when non-default;
// we follow the same convention on submit.

const props = defineProps({
  templateSettings: { type: Object, default: null },
  clientReverseTags: { type: Array, default: () => [] },
});

const STRATEGY_LABELS = {
  random: 'Random',
  roundRobin: 'Round robin',
  leastLoad: 'Least load',
  leastPing: 'Least ping',
};

const rows = computed(() => {
  const list = props.templateSettings?.routing?.balancers || [];
  return list.map((b, idx) => ({
    key: idx,
    tag: b.tag || '',
    strategy: b.strategy?.type || 'random',
    selector: b.selector || [],
    fallbackTag: b.fallbackTag || '',
  }));
});

const outboundTags = computed(() => {
  const tags = new Set();
  for (const o of props.templateSettings?.outbounds || []) {
    if (o.tag) tags.add(o.tag);
  }
  for (const t of props.clientReverseTags || []) {
    if (t) tags.add(t);
  }
  return [...tags];
});

// === Modal state ====================================================
const modalOpen = ref(false);
const editingBalancer = ref(null);
const editingIndex = ref(null);
const otherTags = ref([]);

function tagPool(excludeIdx) {
  return rows.value.filter((b) => b.key !== excludeIdx).map((b) => b.tag).filter(Boolean);
}

function openAdd() {
  editingBalancer.value = null;
  editingIndex.value = null;
  otherTags.value = rows.value.map((b) => b.tag).filter(Boolean);
  modalOpen.value = true;
}
function openEdit(idx) {
  editingBalancer.value = rows.value[idx];
  editingIndex.value = idx;
  otherTags.value = tagPool(idx);
  modalOpen.value = true;
}

function ensureBalancersArray() {
  if (!props.templateSettings.routing) return null;
  if (!Array.isArray(props.templateSettings.routing.balancers)) {
    props.templateSettings.routing.balancers = [];
  }
  return props.templateSettings.routing.balancers;
}

function buildWireBalancer(form) {
  const out = {
    tag: form.tag,
    selector: [...form.selector],
    fallbackTag: form.fallbackTag,
  };
  if (form.strategy && form.strategy !== 'random') {
    out.strategy = { type: form.strategy };
  }
  return out;
}

function onConfirm(form) {
  const arr = ensureBalancersArray();
  if (!arr) return;

  const wire = buildWireBalancer(form);
  if (editingIndex.value == null) {
    arr.push(wire);
  } else {
    const oldTag = arr[editingIndex.value]?.tag;
    arr[editingIndex.value] = wire;
    // Preserve the legacy behaviour: when a balancer's tag is renamed,
    // chase the rename across routing rules so existing references
    // don't dangle.
    if (oldTag && oldTag !== wire.tag) {
      const rules = props.templateSettings.routing.rules || [];
      for (const rule of rules) {
        if (rule?.balancerTag === oldTag) rule.balancerTag = wire.tag;
      }
    }
  }
  modalOpen.value = false;
}

function confirmDelete(idx) {
  Modal.confirm({
    title: `${t('delete')} ${t('pages.xray.Balancers')} #${idx + 1}?`,
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    // Wrap in a block so we discard splice's return value — AD-Vue
    // 4 leaves the modal open if onOk returns a truthy non-thenable
    // (it expects a Promise to await), and splice() returns the array
    // of removed items.
    onOk: () => { props.templateSettings.routing.balancers.splice(idx, 1); },
  });
}

const columns = computed(() => [
  { title: '#', key: 'action', align: 'center', width: 80 },
  { title: 'Tag', dataIndex: 'tag', key: 'tag', align: 'center', width: 160 },
  { title: 'Strategy', key: 'strategy', align: 'center', width: 140 },
  { title: 'Selector', key: 'selector', align: 'center' },
  { title: 'Fallback', dataIndex: 'fallbackTag', key: 'fallbackTag', align: 'center', width: 160 },
]);
</script>

<template>
  <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
    <a-empty v-if="rows.length === 0" :description="t('emptyBalancersDesc')">
      <a-button type="primary" @click="openAdd">
        <template #icon>
          <PlusOutlined />
        </template>
        {{ t('pages.xray.Balancers') }}
      </a-button>
    </a-empty>

    <template v-else>
      <a-button type="primary" @click="openAdd">
        <template #icon>
          <PlusOutlined />
        </template>
        {{ t('pages.xray.Balancers') }}
      </a-button>

      <a-table :columns="columns" :data-source="rows" :row-key="(r) => r.key" :pagination="false" size="small" bordered>
        <template #bodyCell="{ column, record, index }">
          <template v-if="column.key === 'action'">
            <span class="row-index">{{ index + 1 }}</span>
            <a-dropdown :trigger="['click']">
              <a-button shape="circle" size="small" class="action-btn">
                <MoreOutlined />
              </a-button>
              <template #overlay>
                <a-menu>
                  <a-menu-item @click="openEdit(index)">
                    <EditOutlined /> {{ t('edit') }}
                  </a-menu-item>
                  <a-menu-item class="danger" @click="confirmDelete(index)">
                    <DeleteOutlined /> {{ t('delete') }}
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </template>

          <template v-else-if="column.key === 'strategy'">
            <a-tag :color="record.strategy === 'random' ? 'purple' : 'green'">
              {{ STRATEGY_LABELS[record.strategy] || record.strategy }}
            </a-tag>
          </template>

          <template v-else-if="column.key === 'selector'">
            <a-tag v-for="sel in record.selector" :key="sel" class="info-large-tag">{{ sel }}</a-tag>
          </template>
        </template>
      </a-table>
    </template>

    <BalancerFormModal v-model:open="modalOpen" :balancer="editingBalancer" :outbound-tags="outboundTags"
      :other-tags="otherTags" @confirm="onConfirm" />
  </a-space>
</template>

<style scoped>
.row-index {
  font-weight: 500;
  opacity: 0.7;
  margin-right: 6px;
}

.action-btn {
  vertical-align: middle;
}

.danger {
  color: #ff4d4f;
}
</style>
