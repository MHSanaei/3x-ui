<script setup>
import { ref, watch } from 'vue';

// Generic prompt modal — used by features like "import inbound" that
// need a free-form text/textarea input and a confirm callback. The
// parent owns the action; this component only surfaces the value via
// the `confirm` event when the user clicks OK.

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  okText: { type: String, default: 'OK' },
  // 'text' = single-line input; 'textarea' = multi-line.
  type: { type: String, default: 'text', validator: (v) => ['text', 'textarea'].includes(v) },
  initialValue: { type: String, default: '' },
  loading: { type: Boolean, default: false },
});

const emit = defineEmits(['update:open', 'confirm']);

const value = ref('');

watch(() => props.open, (next) => {
  if (next) value.value = props.initialValue;
});

function close() { emit('update:open', false); }
function ok() { emit('confirm', value.value); }

// Enter submits when single-line; ctrl+S submits in textarea mode
// (matches legacy keybindings).
function onKeydown(e) {
  if (props.type !== 'textarea' && e.key === 'Enter') {
    e.preventDefault();
    ok();
    return;
  }
  if (props.type === 'textarea' && e.ctrlKey && e.key.toLowerCase() === 's') {
    e.preventDefault();
    ok();
  }
}
</script>

<template>
  <a-modal :open="open" :title="title" :ok-text="okText" cancel-text="Cancel" :mask-closable="false"
    :confirm-loading="loading" @ok="ok" @cancel="close">
    <a-textarea v-if="type === 'textarea'" v-model:value="value" :auto-size="{ minRows: 10, maxRows: 20 }" autofocus
      @keydown="onKeydown" />
    <a-input v-else v-model:value="value" autofocus @keydown="onKeydown" />
  </a-modal>
</template>
