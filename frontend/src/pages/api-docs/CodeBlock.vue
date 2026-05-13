<script setup>
import { computed, ref } from 'vue';
import { message } from 'ant-design-vue';
import { CopyOutlined, CheckOutlined } from '@ant-design/icons-vue';

const props = defineProps({
  code: { type: String, default: '' },
  lang: { type: String, default: 'json' },
});

const copied = ref(false);

function escapeHtml(str) {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function highlightJson(str) {
  const escaped = escapeHtml(str);
  return escaped.replace(
    /("(?:[^"\\]|\\.)*")\s*(:)|("(?:[^"\\]|\\.)*")|(-?\d+\.?\d*(?:[eE][+-]?\d+)?)\b|(true|false)|(null)|([{}[\]])/g,
    (_m, key, colon, string, number, bool, nil) => {
      if (colon) return `<span class="json-key">${key}</span>${colon}`;
      if (string) return `<span class="json-string">${string}</span>`;
      if (number) return `<span class="json-number">${number}</span>`;
      if (bool) return `<span class="json-boolean">${bool}</span>`;
      if (nil) return `<span class="json-null">${nil}</span>`;
      return _m;
    }
  );
}

const highlighted = computed(() => {
  if (props.lang === 'json') {
    return highlightJson(props.code);
  }
  return escapeHtml(props.code);
});

async function copyCode() {
  try {
    await navigator.clipboard.writeText(props.code);
    copied.value = true;
    message.success('Copied');
    setTimeout(() => { copied.value = false; }, 2000);
  } catch {
    message.error('Copy failed');
  }
}
</script>

<template>
  <div class="code-block-wrapper">
    <button class="copy-btn" :class="{ copied }" @click="copyCode" :title="copied ? 'Copied' : 'Copy'">
      <CheckOutlined v-if="copied" />
      <CopyOutlined v-else />
    </button>
    <pre class="code-block" :class="`lang-${lang}`"><code v-html="highlighted"></code></pre>
  </div>
</template>

<style scoped>
.code-block-wrapper {
  position: relative;
  border-radius: 6px;
  overflow: hidden;
}

.copy-btn {
  position: absolute;
  top: 6px;
  right: 6px;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.85);
  color: rgba(0, 0, 0, 0.5);
  cursor: pointer;
  font-size: 13px;
  opacity: 0;
  transition: opacity 0.15s, background 0.15s, color 0.15s;
}

.code-block-wrapper:hover .copy-btn {
  opacity: 1;
}

.copy-btn:hover {
  background: #fff;
  color: #1677ff;
  border-color: #1677ff;
}

.copy-btn.copied {
  opacity: 1;
  background: #52c41a;
  color: #fff;
  border-color: #52c41a;
}

.code-block {
  background: rgba(128, 128, 128, 0.08);
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 6px;
  padding: 12px;
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-x: auto;
}


</style>

<style>
.json-key { color: #0550ae; }
.json-string { color: #116329; }
.json-number { color: #9a6700; }
.json-boolean { color: #cf222e; }
.json-null { color: #8250df; }

body.dark .code-block {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.88);
}

body.dark .json-key { color: #79c0ff; }
body.dark .json-string { color: #7ee787; }
body.dark .json-number { color: #d29922; }
body.dark .json-boolean { color: #ff7b72; }
body.dark .json-null { color: #d2a8ff; }

body.dark .copy-btn {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.5);
  border-color: rgba(255, 255, 255, 0.15);
}

body.dark .copy-btn:hover {
  background: rgba(255, 255, 255, 0.12);
  color: #58a6ff;
  border-color: #58a6ff;
}
</style>
