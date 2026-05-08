<script setup>
import { computed, reactive, ref, watch } from 'vue';

// Balancer add/edit modal — mirrors xray_balancer_modal.html.
// Tag must be unique across other balancers; selector is a tag-mode
// list constrained to existing outbound tags (but lets users type
// new ones for forward-references).

const props = defineProps({
  open: { type: Boolean, default: false },
  balancer: { type: Object, default: null },
  outboundTags: { type: Array, default: () => [] },
  // All other balancer tags (excludes the one currently being edited)
  // — used for the duplicate-tag check.
  otherTags: { type: Array, default: () => [] },
});

const emit = defineEmits(['update:open', 'confirm']);

const STRATEGIES = [
  { value: 'random', label: 'Random' },
  { value: 'roundRobin', label: 'Round robin' },
  { value: 'leastLoad', label: 'Least load' },
  { value: 'leastPing', label: 'Least ping' },
];

const form = reactive({
  tag: '',
  strategy: 'random',
  selector: [],
  fallbackTag: '',
});
const isEdit = ref(false);

watch(() => props.open, (next) => {
  if (!next) return;
  if (props.balancer) {
    isEdit.value = true;
    form.tag = props.balancer.tag || '';
    form.strategy = props.balancer.strategy || 'random';
    form.selector = [...(props.balancer.selector || [])];
    form.fallbackTag = props.balancer.fallbackTag || '';
  } else {
    isEdit.value = false;
    form.tag = '';
    form.strategy = 'random';
    form.selector = [];
    form.fallbackTag = '';
  }
});

const duplicateTag = computed(
  () => !form.tag || props.otherTags.includes(form.tag),
);
const emptySelector = computed(() => form.selector.length === 0);
const isValid = computed(() => !duplicateTag.value && !emptySelector.value);

function close() { emit('update:open', false); }
function onOk() {
  if (!isValid.value) return;
  emit('confirm', { ...form });
}

const title = computed(() => (isEdit.value ? 'Edit balancer' : 'Add balancer'));
const okText = computed(() => (isEdit.value ? 'Update' : 'Add'));
</script>

<template>
  <a-modal
    :open="open"
    :title="title"
    :ok-text="okText"
    cancel-text="Close"
    :ok-button-props="{ disabled: !isValid }"
    :mask-closable="false"
    @ok="onOk"
    @cancel="close"
  >
    <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
      <a-form-item
        label="Tag"
        :validate-status="duplicateTag ? 'warning' : 'success'"
        has-feedback
      >
        <a-input v-model:value="form.tag" placeholder="unique balancer tag" />
      </a-form-item>

      <a-form-item label="Strategy">
        <a-select v-model:value="form.strategy">
          <a-select-option v-for="s in STRATEGIES" :key="s.value" :value="s.value">{{ s.label }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item
        label="Selector"
        :validate-status="emptySelector ? 'warning' : 'success'"
        has-feedback
      >
        <a-select v-model:value="form.selector" mode="tags" :token-separators="[',']">
          <a-select-option v-for="t in outboundTags" :key="t" :value="t">{{ t }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="Fallback">
        <a-select v-model:value="form.fallbackTag" allow-clear>
          <a-select-option v-for="t in ['', ...outboundTags]" :key="t || '__empty'" :value="t">{{ t || '(none)' }}</a-select-option>
        </a-select>
      </a-form-item>
    </a-form>
  </a-modal>
</template>
