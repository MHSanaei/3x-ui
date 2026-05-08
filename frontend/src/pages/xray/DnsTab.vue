<script setup>
import { computed } from 'vue';

// Compact DNS editor — a master enable switch plus a JSON textarea
// for the full dns + fakedns trees. The legacy panel had a
// dedicated DNS-server modal + fakedns row editor; both are large
// enough to deserve their own commits. For now this gives users a
// working path to edit DNS settings without leaving the structured
// page.

const props = defineProps({
  templateSettings: { type: Object, default: null },
});

const enableDns = computed({
  get: () => !!props.templateSettings?.dns,
  set: (next) => {
    if (!props.templateSettings) return;
    if (next) {
      props.templateSettings.dns = {
        servers: [],
        queryStrategy: 'UseIP',
        tag: 'dns_inbound',
        enableParallelQuery: false,
      };
      props.templateSettings.fakedns = null;
    } else {
      delete props.templateSettings.dns;
      delete props.templateSettings.fakedns;
    }
  },
});

const dnsJson = computed({
  get: () => {
    if (!props.templateSettings?.dns) return '';
    try { return JSON.stringify(props.templateSettings.dns, null, 2); }
    catch (_e) { return ''; }
  },
  set: (next) => {
    if (!props.templateSettings) return;
    try {
      const parsed = next.trim() ? JSON.parse(next) : null;
      props.templateSettings.dns = parsed;
    } catch (_e) {
      // wait for valid JSON — leaves the previous value untouched
    }
  },
});

const fakednsJson = computed({
  get: () => {
    if (!props.templateSettings?.fakedns) return '';
    try { return JSON.stringify(props.templateSettings.fakedns, null, 2); }
    catch (_e) { return ''; }
  },
  set: (next) => {
    if (!props.templateSettings) return;
    try {
      const parsed = next.trim() ? JSON.parse(next) : null;
      if (parsed) props.templateSettings.fakedns = parsed;
      else delete props.templateSettings.fakedns;
    } catch (_e) { /* wait for valid JSON */ }
  },
});
</script>

<template>
  <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
    <a-form layout="vertical">
      <a-form-item label="Enable DNS">
        <a-switch v-model:checked="enableDns" />
      </a-form-item>

      <template v-if="enableDns">
        <a-alert
          type="info"
          show-icon
          message="The full DNS tree is editable here. A dedicated server-by-server editor is coming in a future commit."
          class="mb-12"
        />
        <a-form-item label="dns (JSON)">
          <a-textarea
            v-model:value="dnsJson"
            :auto-size="{ minRows: 12, maxRows: 28 }"
            spellcheck="false"
            class="json-editor"
          />
        </a-form-item>

        <a-form-item label="fakedns (JSON, optional)">
          <a-textarea
            v-model:value="fakednsJson"
            :auto-size="{ minRows: 6, maxRows: 18 }"
            spellcheck="false"
            class="json-editor"
            placeholder="Leave empty to omit fakedns."
          />
        </a-form-item>
      </template>
    </a-form>
  </a-space>
</template>

<style scoped>
.mb-12 { margin-bottom: 12px; }
.json-editor {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}
</style>
