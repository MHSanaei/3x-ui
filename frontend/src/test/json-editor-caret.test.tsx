import { describe, expect, it } from 'vitest';
import { act, render } from '@testing-library/react';
import { EditorView } from '@codemirror/view';

import JsonEditor from '@/components/form/JsonEditor';
import { ThemeProvider } from '@/hooks/useTheme';

describe('JsonEditor external synchronization (#6854)', () => {
  it('preserves the cursor after an external edit to an earlier JSON field', async () => {
    const initial = '{ "tag": "old", "port": 443 }';
    const updated = '{ "tag": "renamed", "port": 443 }';
    const { container, rerender } = render(
      <ThemeProvider>
        <JsonEditor value={initial} />
      </ThemeProvider>,
    );
    const root = container.querySelector<HTMLElement>('.cm-editor');
    if (!root) throw new Error('CodeMirror editor was not mounted');
    const view = EditorView.findFromDOM(root);
    if (!view) throw new Error('CodeMirror view is missing');
    const offsetInPort = 3;

    await act(async () => {
      view.dispatch({ selection: { anchor: initial.indexOf('"port"') + offsetInPort } });
    });
    rerender(
      <ThemeProvider>
        <JsonEditor value={updated} />
      </ThemeProvider>,
    );

    expect(view.state.doc.toString()).toBe(updated);
    expect(view.state.selection.main.head).toBe(updated.indexOf('"port"') + offsetInPort);
  });
});
