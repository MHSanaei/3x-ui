<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PlusOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
} from '@ant-design/icons-vue';
import { Modal } from 'ant-design-vue';

import BalancerFormModal from './BalancerFormModal.vue';
import JsonEditor from '@/components/JsonEditor.vue';

const { t } = useI18n();

// Balancers tab — list + add/edit/delete over
// templateSettings.routing.balancers. The legacy panel kept the wire
// shape's `strategy: { type: 'random' }` nesting only when non-default;
// we follow the same convention on submit.

const props = defineProps({
  templateSettings: { type: Object, default: null },
  clientReverseTags: { type: Array, default: () => [] },
  isMobile: { type: Boolean, default: false },
});

const STRATEGY_LABELS = {
  random: 'Random',
  roundRobin: 'Round robin',
  leastLoad: 'Least load',
  leastPing: 'Least ping',
};

// Observatory defaults — values that the legacy panel seeded when a
// leastPing balancer first appeared. ProbeURL / interval follow Xray's
// own docs (https://xtls.github.io/config/observatory.html).
const DEFAULT_OBSERVATORY = Object.freeze({
  subjectSelector: [],
  probeURL: 'https://www.google.com/generate_204',
  probeInterval: '1m',
  enableConcurrency: true,
});

// BurstObservatory defaults — seeded when a leastLoad balancer is
// configured. Hicloud's generate_204 is the same connectivity probe
// the legacy panel used (https://xtls.github.io/config/burstobservatory.html).
const DEFAULT_BURST_OBSERVATORY = Object.freeze({
  subjectSelector: [],
  pingConfig: {
    destination: 'https://www.google.com/generate_204',
    interval: '1m',
    connectivity: 'http://connectivitycheck.platform.hicloud.com/generate_204',
    timeout: '5s',
    sampling: 2,
  },
});

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

// Keep observatory / burstObservatory in sync with the configured
// balancers. leastPing balancers feed Observatory's subjectSelector;
// leastLoad balancers feed BurstObservatory's. When the matching
// strategy disappears we drop the observatory entirely so the rendered
// xray config stays minimal.
function collectSelectors(list) {
  const out = new Set();
  list.forEach((b) => (b.selector || []).forEach((s) => s && out.add(s)));
  return [...out];
}

function syncObservatories() {
  const t = props.templateSettings;
  if (!t) return;
  const balancers = t.routing?.balancers || [];

  const leastPings = balancers.filter((b) => b.strategy?.type === 'leastPing');
  if (leastPings.length > 0) {
    if (!t.observatory) t.observatory = JSON.parse(JSON.stringify(DEFAULT_OBSERVATORY));
    t.observatory.subjectSelector = collectSelectors(leastPings);
  } else {
    delete t.observatory;
  }

  const leastLoads = balancers.filter((b) => b.strategy?.type === 'leastLoad');
  if (leastLoads.length > 0) {
    if (!t.burstObservatory) {
      t.burstObservatory = JSON.parse(JSON.stringify(DEFAULT_BURST_OBSERVATORY));
    }
    t.burstObservatory.subjectSelector = collectSelectors(leastLoads);
  } else {
    delete t.burstObservatory;
  }
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
  syncObservatories();
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
    onOk: () => {
      props.templateSettings.routing.balancers.splice(idx, 1);
      syncObservatories();
    },
  });
}

const columns = computed(() => [
  { title: '#', key: 'action', align: 'center', width: 100 },
  { title: 'Tag', dataIndex: 'tag', key: 'tag', align: 'center', width: 160 },
  { title: 'Strategy', key: 'strategy', align: 'center', width: 140 },
  { title: 'Selector', key: 'selector', align: 'center' },
  { title: 'Fallback', dataIndex: 'fallbackTag', key: 'fallbackTag', align: 'center', width: 160 },
]);

// === Observatory / BurstObservatory inline editor ====================
// The legacy panel surfaced both top-level observatory blocks here as a
// raw JSON editor so admins could tune probeURL / interval / sampling
// without having to drop into the full xray template tab. We keep that
// affordance but only render it when the matching observatory exists —
// which is itself driven by syncObservatories() above.
const hasObservatory = computed(() => !!props.templateSettings?.observatory);
const hasBurstObservatory = computed(() => !!props.templateSettings?.burstObservatory);
const showObsEditor = computed(() => hasObservatory.value || hasBurstObservatory.value);

const obsView = ref('observatory');

// Keep the radio selection valid as observatories appear/disappear —
// e.g. deleting the last leastPing balancer should flip the editor to
// the burstObservatory pane instead of leaving it pointing at the
// (now-removed) observatory key.
watch(showObsEditor, () => {
  if (obsView.value === 'observatory' && !hasObservatory.value && hasBurstObservatory.value) {
    obsView.value = 'burstObservatory';
  } else if (obsView.value === 'burstObservatory' && !hasBurstObservatory.value && hasObservatory.value) {
    obsView.value = 'observatory';
  }
}, { immediate: true });

const obsText = computed({
  get: () => {
    const t = props.templateSettings;
    if (!t) return '';
    const src = obsView.value === 'observatory' ? t.observatory : t.burstObservatory;
    return src ? JSON.stringify(src, null, 2) : '';
  },
  set: (next) => {
    let parsed;
    try { parsed = JSON.parse(next); } catch (_e) { return; }
    if (!props.templateSettings) return;
    if (obsView.value === 'observatory') {
      props.templateSettings.observatory = parsed;
    } else {
      props.templateSettings.burstObservatory = parsed;
    }
  },
});
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

      <a-table :columns="columns" :data-source="rows" :row-key="(r) => r.key" :pagination="false"
        size="small" :scroll="{ x: 400 }">
        <template #bodyCell="{ column, record, index }">
          <template v-if="column.key === 'action'">
            <div class="action-cell">
              <span class="row-index">{{ index + 1 }}</span>

              <div :class="!isMobile ? 'action-buttons' : ''">
                <a-button v-if="!isMobile" shape="circle" size="small" @click="openEdit(index)">
                  <template #icon>
                    <EditOutlined />
                  </template>
                </a-button>

                <a-dropdown :trigger="['click']">
                  <a-button shape="circle" size="small">
                    <template #icon>
                      <MoreOutlined />
                    </template>
                  </a-button>
                  <template #overlay>
                    <a-menu>
                      <a-menu-item v-if="isMobile" @click="openEdit(index)">
                        <EditOutlined /> {{ t('edit') }}
                      </a-menu-item>
                      <a-menu-item class="danger" @click="confirmDelete(index)">
                        <DeleteOutlined /> {{ t('delete') }}
                      </a-menu-item>
                    </a-menu>
                  </template>
                </a-dropdown>
              </div>
            </div>
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

      <template v-if="showObsEditor">
        <a-divider :style="{ margin: '8px 0' }" />
        <a-radio-group v-model:value="obsView" button-style="solid" size="small">
          <a-radio-button v-if="hasObservatory" value="observatory">Observatory</a-radio-button>
          <a-radio-button v-if="hasBurstObservatory" value="burstObservatory">Burst Observatory</a-radio-button>
        </a-radio-group>
        <JsonEditor v-model:value="obsText" min-height="220px" max-height="480px" />
      </template>
    </template>

    <BalancerFormModal v-model:open="modalOpen" :balancer="editingBalancer" :outbound-tags="outboundTags"
      :other-tags="otherTags" @confirm="onConfirm" />
  </a-space>
</template>

<style scoped>
.action-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.row-index {
  font-weight: 500;
  opacity: 0.7;
  min-width: 18px;
  text-align: right;
}

.action-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  margin-left: auto;
}

.danger {
  color: #ff4d4f;
}

</style>
