<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { EditorView, basicSetup } from 'codemirror';
import { EditorState, Compartment } from '@codemirror/state';
import { json, jsonParseLinter } from '@codemirror/lang-json';
import { lintGutter, linter } from '@codemirror/lint';
import { oneDarkHighlightStyle } from '@codemirror/theme-one-dark';
import { syntaxHighlighting } from '@codemirror/language';
import { keymap } from '@codemirror/view';
import { indentWithTab } from '@codemirror/commands';

import { theme as themeState } from '@/composables/useTheme.js';

const props = defineProps({
  value: { type: String, default: '' },
  minHeight: { type: String, default: '320px' },
  maxHeight: { type: String, default: '600px' },
  readonly: { type: Boolean, default: false },
});

const emit = defineEmits(['update:value', 'change']);

const host = ref(null);
let view = null;
const themeCompartment = new Compartment();
const readonlyCompartment = new Compartment();

function buildDarkTheme({ bg, panelBg, activeBg, border, selection }) {
  return EditorView.theme(
    {
      '&': { color: '#dcdcdc', backgroundColor: bg },
      '.cm-content': { caretColor: '#dcdcdc' },
      '.cm-cursor, .cm-dropCursor': { borderLeftColor: '#dcdcdc' },
      '.cm-gutters': {
        backgroundColor: bg,
        borderRight: `1px solid ${border}`,
        color: '#6a6a6a',
      },
      '.cm-activeLine': { backgroundColor: activeBg },
      '.cm-activeLineGutter': { backgroundColor: activeBg, color: '#dcdcdc' },
      '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection':
        { backgroundColor: selection },
      '.cm-panels': { backgroundColor: panelBg, color: '#dcdcdc' },
      '.cm-panels.cm-panels-top': { borderBottom: `1px solid ${border}` },
      '.cm-panels.cm-panels-bottom': { borderTop: `1px solid ${border}` },
      '.cm-tooltip': {
        backgroundColor: panelBg,
        border: `1px solid ${border}`,
        color: '#dcdcdc',
      },
    },
    { dark: true },
  );
}

const darkTheme = buildDarkTheme({
  bg: '#1e1e1e',
  panelBg: '#2d2d30',
  activeBg: '#252526',
  border: '#3a3a3c',
  selection: '#3a3a3c',
});

const ultraDarkTheme = buildDarkTheme({
  bg: '#0a0a0a',
  panelBg: '#141414',
  activeBg: '#141414',
  border: '#1f1f1f',
  selection: '#2a2a2a',
});

function themeExtension() {
  if (!themeState.isDark) return [];
  const chrome = themeState.isUltra ? ultraDarkTheme : darkTheme;
  return [chrome, syntaxHighlighting(oneDarkHighlightStyle)];
}

function readonlyExtension() {
  return EditorState.readOnly.of(props.readonly);
}

onMounted(() => {
  const updateListener = EditorView.updateListener.of((u) => {
    if (!u.docChanged) return;
    const next = u.state.doc.toString();
    if (next === props.value) return;
    emit('update:value', next);
    emit('change', next);
  });

  view = new EditorView({
    parent: host.value,
    state: EditorState.create({
      doc: props.value || '',
      extensions: [
        basicSetup,
        keymap.of([indentWithTab]),
        json(),
        linter(jsonParseLinter()),
        lintGutter(),
        EditorView.lineWrapping,
        updateListener,
        themeCompartment.of(themeExtension()),
        readonlyCompartment.of(readonlyExtension()),
        EditorView.theme({
          '&': { height: '100%' },
          '.cm-scroller': {
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
            fontSize: '12px',
            minHeight: props.minHeight,
            maxHeight: props.maxHeight,
          },
        }),
      ],
    }),
  });
});

watch(() => props.value, (next) => {
  if (!view) return;
  const current = view.state.doc.toString();
  if (next === current) return;
  view.dispatch({
    changes: { from: 0, to: current.length, insert: next || '' },
  });
});

watch(
  [() => themeState.isDark, () => themeState.isUltra],
  () => {
    if (!view) return;
    view.dispatch({ effects: themeCompartment.reconfigure(themeExtension()) });
  },
);

watch(
  () => props.readonly,
  () => {
    if (!view) return;
    view.dispatch({ effects: readonlyCompartment.reconfigure(readonlyExtension()) });
  },
);

onBeforeUnmount(() => {
  view?.destroy();
  view = null;
});

defineExpose({
  focus: () => view?.focus(),
});
</script>

<template>
  <div ref="host" class="json-editor-host" />
</template>

<style scoped>
.json-editor-host {
  border: 1px solid var(--ant-color-border, #d9d9d9);
  border-radius: 6px;
  overflow: hidden;
  background: var(--ant-color-bg-container, #fff);
}

.json-editor-host :deep(.cm-editor),
.json-editor-host :deep(.cm-editor.cm-focused) {
  outline: none;
}

.json-editor-host:focus-within {
  border-color: var(--ant-color-primary, #1677ff);
  box-shadow: 0 0 0 2px rgba(22, 119, 255, 0.1);
}

:global(body.dark) .json-editor-host {
  border-color: #3a3a3c;
  background: #1e1e1e;
}

:global(html[data-theme="ultra-dark"]) .json-editor-host {
  border-color: #1f1f1f;
  background: #0a0a0a;
}
</style>
