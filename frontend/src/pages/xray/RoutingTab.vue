<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PlusOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  ExportOutlined,
  ClusterOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  HolderOutlined,
} from '@ant-design/icons-vue';
import { Modal } from 'ant-design-vue';

import RuleFormModal from './RuleFormModal.vue';

const { t } = useI18n();

// Routing tab — table over templateSettings.routing.rules with the
// modernised legacy column layout. Each row is rendered as a single
// "lead value + N more" pill per criterion (matches the legacy pill
// layout); full lists surface via tooltip on hover.
//
// Reorder via Pointer Events on the grip icon — unified mouse +
// touch + pen path so the same code works on desktop and mobile
// (HTML5 drag doesn't fire from touch on iOS Safari, hence the
// switch). Up/down buttons in the action menu stay as a keyboard
// fallback. The mobile column layout drops source/network/
// destination criteria for readability.

const props = defineProps({
  templateSettings: { type: Object, default: null },
  inboundTags: { type: Array, default: () => [] },
  clientReverseTags: { type: Array, default: () => [] },
  isMobile: { type: Boolean, default: false },
});

// === Table data — match the legacy routingRuleData shape ============
// Convert array criteria to CSV strings so the pill renderer can
// split + summarise them without needing a separate path per shape.
const rows = computed(() => {
  if (!props.templateSettings?.routing?.rules) return [];
  return props.templateSettings.routing.rules.map((rule, idx) => {
    const r = { key: idx, ...rule };
    if (Array.isArray(r.domain)) r.domain = r.domain.join(',');
    if (Array.isArray(r.ip)) r.ip = r.ip.join(',');
    if (Array.isArray(r.source)) r.source = r.source.join(',');
    if (Array.isArray(r.user)) r.user = r.user.join(',');
    if (Array.isArray(r.inboundTag)) r.inboundTag = r.inboundTag.join(',');
    if (Array.isArray(r.protocol)) r.protocol = r.protocol.join(',');
    if (r.attrs && typeof r.attrs === 'object' && !Array.isArray(r.attrs)) {
      r.attrs = JSON.stringify(r.attrs, null, 2);
    }
    return r;
  });
});

function csv(value) {
  if (!value) return [];
  return String(value).split(',').map((s) => s.trim()).filter(Boolean);
}

// === Modal state ====================================================
const ruleModalOpen = ref(false);
const editingRule = ref(null);
const editingIndex = ref(null);

const inboundTagOptions = computed(() => {
  const seen = new Set();
  const out = [];

  function pushUnique(tag) {
    if (!tag) return;
    if (seen.has(tag)) return;
    seen.add(tag);
    out.push(tag);
  }

  for (const ib of props.templateSettings?.inbounds || []) {
    pushUnique(ib.tag);
  }
  for (const t of props.inboundTags || []) {
    pushUnique(t);
  }
  for (const ob of props.templateSettings?.outbounds || []) {
    const rt = ob?.reverse?.tag || ob?.settings?.reverse?.tag || ob?.settings?.inboundTag;
    pushUnique(rt);
  }
  pushUnique(props.templateSettings?.dns?.tag);

  for (const s of props.templateSettings?.dns?.servers || []) {
    if (typeof s === 'object' && s?.tag) pushUnique(s.tag);
  }

  return [...out];
});

const outboundTagOptions = computed(() => {
  const out = new Set(['']);
  for (const ob of props.templateSettings?.outbounds || []) {
    if (ob.tag) out.add(ob.tag);
  }
  for (const t of props.clientReverseTags || []) {
    if (t) out.add(t);
  }
  return [...out];
});

const balancerTagOptions = computed(() => {
  const out = [''];
  for (const b of props.templateSettings?.routing?.balancers || []) {
    if (b.tag) out.push(b.tag);
  }
  return out;
});

function openAdd() {
  editingRule.value = null;
  editingIndex.value = null;
  ruleModalOpen.value = true;
}

function openEdit(idx) {
  editingRule.value = props.templateSettings.routing.rules[idx];
  editingIndex.value = idx;
  ruleModalOpen.value = true;
}

function onRuleConfirm(rule) {
  // Empty submit (e.g. user clears every field) collapses to an
  // object with only `type: "field"`. Match legacy: skip the write
  // when the result is essentially empty.
  if (JSON.stringify(rule).length <= 3) {
    ruleModalOpen.value = false;
    return;
  }
  if (editingIndex.value == null) {
    props.templateSettings.routing.rules.push(rule);
  } else {
    props.templateSettings.routing.rules[editingIndex.value] = rule;
  }
  ruleModalOpen.value = false;
}

function confirmDelete(idx) {
  Modal.confirm({
    title: `${t('delete')} ${t('pages.xray.Routings')} #${idx + 1}?`,
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: () => { props.templateSettings.routing.rules.splice(idx, 1); },
  });
}

function moveUp(idx) {
  if (idx <= 0) return;
  const rules = props.templateSettings.routing.rules;
  [rules[idx - 1], rules[idx]] = [rules[idx], rules[idx - 1]];
}
function moveDown(idx) {
  const rules = props.templateSettings.routing.rules;
  if (idx >= rules.length - 1) return;
  [rules[idx + 1], rules[idx]] = [rules[idx], rules[idx + 1]];
}

const draggedIndex = ref(null);
const dropTargetIndex = ref(null);
let dragStartY = 0;
let dragMoved = false;

function onHandlePointerDown(idx, ev) {
  if (ev.button != null && ev.button !== 0) return;
  ev.preventDefault();
  draggedIndex.value = idx;
  dropTargetIndex.value = idx;
  dragStartY = ev.clientY;
  dragMoved = false;
  document.addEventListener('pointermove', onDragPointerMove);
  document.addEventListener('pointerup', onDragPointerUp);
  document.addEventListener('pointercancel', onDragPointerUp);
}

function onDragPointerMove(ev) {
  if (draggedIndex.value == null) return;
  if (!dragMoved && Math.abs(ev.clientY - dragStartY) < 5) return;
  dragMoved = true;
  const el = document.elementFromPoint(ev.clientX, ev.clientY);
  if (!el) return;
  const target = el.closest('[data-row-key]');
  if (!target) return;
  const idx = Number(target.getAttribute('data-row-key'));
  if (Number.isFinite(idx)) dropTargetIndex.value = idx;
}

function onDragPointerUp() {
  document.removeEventListener('pointermove', onDragPointerMove);
  document.removeEventListener('pointerup', onDragPointerUp);
  document.removeEventListener('pointercancel', onDragPointerUp);
  const from = draggedIndex.value;
  const to = dropTargetIndex.value;
  draggedIndex.value = null;
  dropTargetIndex.value = null;
  if (!dragMoved || from == null || to == null || from === to) return;
  const rules = props.templateSettings.routing.rules;
  const [moved] = rules.splice(from, 1);
  rules.splice(to, 0, moved);
}

function rowProps(_record, index) {
  const classes = [];
  if (draggedIndex.value === index) classes.push('row-dragging');
  if (dropTargetIndex.value === index && draggedIndex.value !== index) {
    classes.push(index > draggedIndex.value ? 'drop-after' : 'drop-before');
  }
  return { class: classes.join(' ') };
}

// === Columns =========================================================
// Computed so titles re-render after a locale swap.
const desktopColumns = computed(() => [
  { title: '#', align: 'center', width: 100, key: 'action' },
  { title: 'Source', align: 'left', width: 180, key: 'source' },
  { title: t('pages.inbounds.network'), align: 'left', width: 180, key: 'network' },
  { title: 'Destination', align: 'left', key: 'destination' },
  { title: t('pages.xray.Inbounds'), align: 'left', width: 180, key: 'inbound' },
  { title: t('pages.xray.Outbounds'), align: 'left', width: 170, key: 'outbound' },
  { title: t('pages.xray.Balancers'), align: 'left', width: 150, key: 'balancer' },
]);
const columns = computed(() => desktopColumns.value);

function ruleCriteriaChips(rule) {
  const chips = [];
  if (rule.domain) chips.push({ label: 'Domain', value: rule.domain });
  if (rule.ip) chips.push({ label: 'IP', value: rule.ip });
  if (rule.port) chips.push({ label: 'Port', value: rule.port });
  if (rule.sourceIP) chips.push({ label: 'Src IP', value: rule.sourceIP });
  if (rule.sourcePort) chips.push({ label: 'Src Port', value: rule.sourcePort });
  if (rule.network) chips.push({ label: 'L4', value: rule.network });
  if (rule.protocol) chips.push({ label: 'Protocol', value: rule.protocol });
  if (rule.user) chips.push({ label: 'User', value: rule.user });
  if (rule.vlessRoute) chips.push({ label: 'VLESS', value: rule.vlessRoute });
  return chips;
}

function chipPreview(value) {
  const parts = csv(value);
  if (parts.length === 0) return '';
  if (parts.length === 1) return parts[0];
  return `${parts[0]} +${parts.length - 1}`;
}
</script>

<template>
  <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
    <a-button type="primary" @click="openAdd">
      <template #icon>
        <PlusOutlined />
      </template>
      {{ t('pages.xray.Routings') }}
    </a-button>

    <!-- Mobile: stacked cards. The desktop a-table doesn't fit on a
         phone (~520px of columns alone), so render each rule as a
         compact card with the routing summary + criteria chips. -->
    <div v-if="isMobile" class="rule-list">
      <div v-for="(rule, index) in rows" :key="rule.key" class="rule-card" :class="{
        'row-dragging': draggedIndex === index,
        'drop-before': dropTargetIndex === index && draggedIndex != null && index < draggedIndex,
        'drop-after': dropTargetIndex === index && draggedIndex != null && index > draggedIndex,
      }" :data-row-key="index">
        <div class="rule-card-head">
          <HolderOutlined class="drag-handle" @pointerdown="onHandlePointerDown(index, $event)" />
          <span class="rule-number">#{{ index + 1 }}</span>
          <a-dropdown :trigger="['click']">
            <a-button shape="circle" size="small">
              <MoreOutlined />
            </a-button>
            <template #overlay>
              <a-menu>
                <a-menu-item @click="openEdit(index)">
                  <EditOutlined /> {{ t('edit') }}
                </a-menu-item>
                <a-menu-item :disabled="index === 0" @click="moveUp(index)">
                  <ArrowUpOutlined />
                </a-menu-item>
                <a-menu-item :disabled="index === rows.length - 1" @click="moveDown(index)">
                  <ArrowDownOutlined />
                </a-menu-item>
                <a-menu-item class="danger" @click="confirmDelete(index)">
                  <DeleteOutlined /> {{ t('delete') }}
                </a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </div>

        <div class="rule-flow">
          <div class="flow-side">
            <span class="flow-label">{{ t('pages.xray.Inbounds') }}</span>
            <a-tag v-if="rule.inboundTag" color="blue" class="flow-tag">
              {{ chipPreview(rule.inboundTag) }}
            </a-tag>
            <span v-else class="criterion-empty">any</span>
          </div>
          <span class="flow-arrow">→</span>
          <div class="flow-side flow-side-target">
            <span class="flow-label">{{
              rule.balancerTag ? (t('pages.xray.balancer') || 'Balancer') : t('pages.xray.Outbounds')
            }}</span>
            <a-tag v-if="rule.outboundTag" color="green" class="flow-tag">
              <ExportOutlined /> {{ rule.outboundTag }}
            </a-tag>
            <a-tag v-else-if="rule.balancerTag" color="purple" class="flow-tag">
              <ClusterOutlined /> {{ rule.balancerTag }}
            </a-tag>
            <span v-else class="criterion-empty">—</span>
          </div>
        </div>

        <div v-if="ruleCriteriaChips(rule).length" class="rule-criteria">
          <a-tooltip v-for="chip in ruleCriteriaChips(rule)" :key="chip.label" :title="`${chip.label}: ${chip.value}`">
            <span class="criterion-chip">
              <span class="criterion-chip-label">{{ chip.label }}</span>
              <span class="criterion-chip-value">{{ chipPreview(chip.value) }}</span>
            </span>
          </a-tooltip>
        </div>
      </div>
      <div v-if="!rows.length" class="rule-empty">—</div>
    </div>

    <a-table v-else :columns="columns" :data-source="rows" :row-key="(r) => r.key" :pagination="false"
      :scroll="{ x: 1150 }" size="small" class="routing-table" :custom-row="rowProps">
      <template #bodyCell="{ column, record, index }">
        <!-- ============== # / actions ============== -->
        <template v-if="column.key === 'action'">
          <div class="action-cell">
            <HolderOutlined class="drag-handle" :title="t('drag') || 'Drag to reorder'"
              @pointerdown="onHandlePointerDown(index, $event)" />
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
                    <a-menu-item :disabled="index === 0" @click="moveUp(index)">
                      <ArrowUpOutlined />
                    </a-menu-item>
                    <a-menu-item :disabled="index === rows.length - 1" @click="moveDown(index)">
                      <ArrowDownOutlined />
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

        <!-- ============== Source ============== -->
        <template v-else-if="column.key === 'source'">
          <div class="criterion-flow">
            <a-tooltip v-if="record.sourceIP" :title="`Source IP: ${record.sourceIP}`">
              <span class="criterion-row">
                <span class="criterion-label">IP</span>
                <span class="criterion-value">{{ csv(record.sourceIP)[0] }}</span>
                <span v-if="csv(record.sourceIP).length > 1" class="criterion-more">+{{ csv(record.sourceIP).length - 1
                }}</span>
              </span>
            </a-tooltip>
            <a-tooltip v-if="record.sourcePort" :title="`Source port: ${record.sourcePort}`">
              <span class="criterion-row">
                <span class="criterion-label">Port</span>
                <span class="criterion-value">{{ csv(record.sourcePort)[0] }}</span>
                <span v-if="csv(record.sourcePort).length > 1" class="criterion-more">+{{ csv(record.sourcePort).length
                  - 1 }}</span>
              </span>
            </a-tooltip>
            <a-tooltip v-if="record.vlessRoute" :title="`VLESS route: ${record.vlessRoute}`">
              <span class="criterion-row">
                <span class="criterion-label">VLESS</span>
                <span class="criterion-value">{{ csv(record.vlessRoute)[0] }}</span>
                <span v-if="csv(record.vlessRoute).length > 1" class="criterion-more">+{{ csv(record.vlessRoute).length
                  - 1 }}</span>
              </span>
            </a-tooltip>
            <span v-if="!record.sourceIP && !record.sourcePort && !record.vlessRoute" class="criterion-empty">—</span>
          </div>
        </template>

        <!-- ============== Network ============== -->
        <template v-else-if="column.key === 'network'">
          <div class="criterion-flow">
            <a-tooltip v-if="record.network" :title="`L4: ${record.network}`">
              <span class="criterion-row">
                <span class="criterion-label">L4</span>
                <span class="criterion-value">{{ csv(record.network)[0] }}</span>
                <span v-if="csv(record.network).length > 1" class="criterion-more">+{{ csv(record.network).length - 1
                }}</span>
              </span>
            </a-tooltip>
            <a-tooltip v-if="record.protocol" :title="`Protocol: ${record.protocol}`">
              <span class="criterion-row">
                <span class="criterion-label">Protocol</span>
                <span class="criterion-value">{{ csv(record.protocol)[0] }}</span>
                <span v-if="csv(record.protocol).length > 1" class="criterion-more">+{{ csv(record.protocol).length - 1
                }}</span>
              </span>
            </a-tooltip>
            <a-tooltip v-if="record.attrs" :title="`Attrs: ${record.attrs}`">
              <span class="criterion-row">
                <span class="criterion-label">Attrs</span>
                <span class="criterion-value">{{ csv(record.attrs)[0] }}</span>
              </span>
            </a-tooltip>
            <span v-if="!record.network && !record.protocol && !record.attrs" class="criterion-empty">—</span>
          </div>
        </template>

        <!-- ============== Destination ============== -->
        <template v-else-if="column.key === 'destination'">
          <div class="criterion-flow">
            <a-tooltip v-if="record.ip" :title="`Destination IP: ${record.ip}`">
              <span class="criterion-row">
                <span class="criterion-label">IP</span>
                <span class="criterion-value">{{ csv(record.ip)[0] }}</span>
                <span v-if="csv(record.ip).length > 1" class="criterion-more">+{{ csv(record.ip).length - 1 }}</span>
              </span>
            </a-tooltip>
            <a-tooltip v-if="record.domain" :title="`Domain: ${record.domain}`">
              <span class="criterion-row">
                <span class="criterion-label">Domain</span>
                <span class="criterion-value">{{ csv(record.domain)[0] }}</span>
                <span v-if="csv(record.domain).length > 1" class="criterion-more">+{{ csv(record.domain).length - 1
                }}</span>
              </span>
            </a-tooltip>
            <a-tooltip v-if="record.port" :title="`Destination port: ${record.port}`">
              <span class="criterion-row">
                <span class="criterion-label">Port</span>
                <span class="criterion-value">{{ csv(record.port)[0] }}</span>
                <span v-if="csv(record.port).length > 1" class="criterion-more">+{{ csv(record.port).length - 1
                }}</span>
              </span>
            </a-tooltip>
            <span v-if="!record.ip && !record.domain && !record.port" class="criterion-empty">—</span>
          </div>
        </template>

        <!-- ============== Inbound ============== -->
        <template v-else-if="column.key === 'inbound'">
          <div class="criterion-flow">
            <a-tooltip v-if="record.inboundTag" :title="`Inbound tag: ${record.inboundTag}`">
              <span class="criterion-row">
                <span class="criterion-label">Tag</span>
                <span class="criterion-value">{{ csv(record.inboundTag)[0] }}</span>
                <span v-if="csv(record.inboundTag).length > 1" class="criterion-more">+{{ csv(record.inboundTag).length
                  - 1 }}</span>
              </span>
            </a-tooltip>
            <a-tooltip v-if="record.user" :title="`User: ${record.user}`">
              <span class="criterion-row">
                <span class="criterion-label">User</span>
                <span class="criterion-value">{{ csv(record.user)[0] }}</span>
                <span v-if="csv(record.user).length > 1" class="criterion-more">+{{ csv(record.user).length - 1
                }}</span>
              </span>
            </a-tooltip>
            <span v-if="!record.inboundTag && !record.user" class="criterion-empty">—</span>
          </div>
        </template>

        <!-- ============== Outbound ============== -->
        <template v-else-if="column.key === 'outbound'">
          <div class="target-cell">
            <div v-if="record.outboundTag" class="target-row">
              <ExportOutlined class="target-icon" />
              <a-tag color="green">{{ record.outboundTag }}</a-tag>
            </div>
            <span v-else class="criterion-empty">—</span>
          </div>
        </template>

        <!-- ============== Balancer ============== -->
        <template v-else-if="column.key === 'balancer'">
          <div class="target-cell">
            <div v-if="record.balancerTag" class="target-row">
              <ClusterOutlined class="target-icon" />
              <a-tag color="purple">{{ record.balancerTag }}</a-tag>
            </div>
            <span v-else class="criterion-empty">—</span>
          </div>
        </template>
      </template>
    </a-table>

    <RuleFormModal v-model:open="ruleModalOpen" :rule="editingRule" :inbound-tags="inboundTagOptions"
      :outbound-tags="outboundTagOptions" :balancer-tags="balancerTagOptions" @confirm="onRuleConfirm" />
  </a-space>
</template>

<style scoped>
.action-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.drag-handle {
  cursor: grab;
  opacity: 0.35;
  font-size: 14px;
  padding: 4px;
  margin: -4px;
  touch-action: none;
  transition: opacity 0.15s;
}

.drag-handle:hover {
  opacity: 0.8;
}

.drag-handle:active {
  cursor: grabbing;
}

:deep(.row-dragging) {
  opacity: 0.4;
}

:deep(.drop-before > td) {
  box-shadow: inset 0 2px 0 0 #1677ff;
}

:deep(.drop-after > td) {
  box-shadow: inset 0 -2px 0 0 #1677ff;
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

.criterion-flow {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}

.criterion-row {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.criterion-label {
  font-size: 10px;
  text-transform: uppercase;
  opacity: 0.55;
  letter-spacing: 0.04em;
}

.criterion-value {
  font-weight: 500;
}

.criterion-more {
  font-size: 11px;
  padding: 0 5px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.06);
}

:global(body.dark) .criterion-more {
  background: rgba(255, 255, 255, 0.1);
}

.criterion-empty {
  opacity: 0.4;
}

.target-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.target-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.target-icon {
  font-size: 12px;
  opacity: 0.6;
}

.danger {
  color: #ff4d4f;
}

/* === Mobile card list ====================================== */
.rule-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rule-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  background: var(--bg-card, #fff);
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 8px;
  transition: opacity 0.15s, box-shadow 0.15s;
}

.rule-card.row-dragging {
  opacity: 0.4;
}

.rule-card.drop-before {
  box-shadow: inset 0 2px 0 0 #1677ff;
}

.rule-card.drop-after {
  box-shadow: inset 0 -2px 0 0 #1677ff;
}

.rule-card-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.rule-number {
  font-weight: 600;
  font-size: 13px;
  opacity: 0.75;
  flex: 1;
}

.rule-flow {
  display: flex;
  align-items: center;
  gap: 8px;
}

.flow-side {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.flow-side-target {
  align-items: flex-end;
  text-align: right;
}

.flow-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  opacity: 0.55;
}

.flow-tag {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin: 0;
}

.flow-arrow {
  font-size: 16px;
  opacity: 0.45;
  flex-shrink: 0;
}

.rule-criteria {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding-top: 6px;
  border-top: 1px dashed rgba(128, 128, 128, 0.2);
}

.criterion-chip {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  padding: 1px 6px;
  font-size: 11px;
  background: rgba(128, 128, 128, 0.08);
  border-radius: 4px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.criterion-chip-label {
  font-size: 9px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  opacity: 0.6;
}

.criterion-chip-value {
  font-weight: 500;
}

.rule-empty {
  padding: 24px;
  text-align: center;
  opacity: 0.4;
}

:global(body.dark) .rule-card {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.08);
}

:global(body.dark) .criterion-chip {
  background: rgba(255, 255, 255, 0.06);
}
</style>
