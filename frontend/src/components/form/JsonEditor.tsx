import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { EditorView, basicSetup } from 'codemirror';
import { EditorState, Compartment } from '@codemirror/state';
import { json, jsonParseLinter } from '@codemirror/lang-json';
import { lintGutter, linter } from '@codemirror/lint';
import { oneDarkHighlightStyle } from '@codemirror/theme-one-dark';
import { syntaxHighlighting } from '@codemirror/language';
import { keymap } from '@codemirror/view';
import { indentWithTab } from '@codemirror/commands';

import { useTheme } from '@/hooks/useTheme';
import './JsonEditor.css';

export interface JsonEditorProps {
  value: string;
  onChange?: (next: string) => void;
  minHeight?: string;
  maxHeight?: string;
  readOnly?: boolean;
}

export interface JsonEditorHandle {
  focus: () => void;
}

interface DarkPalette {
  bg: string;
  panelBg: string;
  activeBg: string;
  border: string;
  selection: string;
}

function buildDarkTheme({ bg, panelBg, activeBg, border, selection }: DarkPalette) {
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

function themeExtension(isDark: boolean, isUltra: boolean) {
  if (!isDark) return [];
  const chrome = isUltra ? ultraDarkTheme : darkTheme;
  return [chrome, syntaxHighlighting(oneDarkHighlightStyle)];
}

const JsonEditor = forwardRef<JsonEditorHandle, JsonEditorProps>(function JsonEditor(
  { value, onChange, minHeight = '320px', maxHeight = '600px', readOnly = false },
  ref,
) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const viewRef = useRef<EditorView | null>(null);
  const themeCompartmentRef = useRef<Compartment>(new Compartment());
  const readonlyCompartmentRef = useRef<Compartment>(new Compartment());
  const onChangeRef = useRef(onChange);
  const valueRef = useRef(value);
  const { isDark, isUltra } = useTheme();
  const { t } = useTranslation();

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useImperativeHandle(ref, () => ({
    focus: () => viewRef.current?.focus(),
  }));

  useEffect(() => {
    if (!hostRef.current) return;

    const updateListener = EditorView.updateListener.of((u) => {
      if (!u.docChanged) return;
      const next = u.state.doc.toString();
      if (next === valueRef.current) return;
      valueRef.current = next;
      onChangeRef.current?.(next);
    });

    const view = new EditorView({
      parent: hostRef.current,
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          EditorView.contentAttributes.of({ 'aria-label': t('jsonEditor') }),
          keymap.of([indentWithTab]),
          json(),
          linter(jsonParseLinter()),
          lintGutter(),
          EditorView.lineWrapping,
          updateListener,
          themeCompartmentRef.current.of(themeExtension(isDark, isUltra)),
          readonlyCompartmentRef.current.of(EditorState.readOnly.of(readOnly)),
          EditorView.theme({
            '&': { height: '100%' },
            '.cm-scroller': {
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
              fontSize: '12px',
              minHeight,
              maxHeight,
            },
          }),
        ],
      }),
    });

    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (value === current) return;
    valueRef.current = value;
    view.dispatch({ changes: { from: 0, to: current.length, insert: value } });
  }, [value]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: themeCompartmentRef.current.reconfigure(themeExtension(isDark, isUltra)),
    });
  }, [isDark, isUltra]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: readonlyCompartmentRef.current.reconfigure(EditorState.readOnly.of(readOnly)),
    });
  }, [readOnly]);

  return <div ref={hostRef} className="json-editor-host" aria-label={t('jsonEditor')} />;
});

export default JsonEditor;
