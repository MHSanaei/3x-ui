import type { TFunction } from 'i18next';

import enUS from '../../../../internal/web/translation/en-US.json';

const labelKeys = new Map(Object.entries(enUS.apiDocs.labels).map(([key, text]) => [text, key]));
const untouched =
  'pre, code, textarea, .microlight, .model-title, .prop-name, .parameter__name:not(h4), .parameter__enum, .prop-enum, .opblock-summary-path';
const attributes = ['aria-label', 'title', 'placeholder'];

// Swagger has no message catalog. Change only exact display labels, preserving
// text nodes, event handlers, editable values and every code/example container.
export function bindSwaggerLabels(root: HTMLElement, t: TFunction): () => void {
  function translated(value: string): string {
    const normalized = value.replace(/\s+/g, ' ').trim();
    const key = labelKeys.get(normalized);
    if (!key) return value;
    const edges = value.match(/^(\s*)([\s\S]*?)(\s*)$/)!;
    return edges[1] + t('apiDocs.labels.' + key, { defaultValue: normalized }) + edges[3];
  }

  function text(node: Node) {
    if (
      node.nodeType !== Node.TEXT_NODE ||
      !node.parentElement ||
      node.parentElement.closest(untouched)
    )
      return;
    const before = node.nodeValue ?? '';
    const after = translated(before);
    if (after !== before) node.nodeValue = after;
  }

  function attrs(element: Element) {
    if (element.closest(untouched)) return;
    for (const name of attributes) {
      const before = element.getAttribute(name);
      if (before === null) continue;
      const after = translated(before);
      if (after !== before) element.setAttribute(name, after);
    }
  }

  function scan(node: Node) {
    if (node.nodeType === Node.TEXT_NODE) return text(node);
    if (!(node instanceof Element)) return;
    attrs(node);
    for (const element of node.querySelectorAll('[aria-label], [title], [placeholder]'))
      attrs(element);
    const walker = root.ownerDocument.createTreeWalker(node, NodeFilter.SHOW_TEXT);
    let child;
    while ((child = walker.nextNode())) text(child);
  }

  scan(root);
  const observer = new MutationObserver((changes) => {
    for (const change of changes) {
      if (change.type === 'childList') change.addedNodes.forEach(scan);
      else if (change.type === 'characterData') text(change.target);
      else if (change.target instanceof Element) attrs(change.target);
    }
  });
  observer.observe(root, {
    subtree: true,
    childList: true,
    characterData: true,
    attributes: true,
    attributeFilter: attributes,
  });
  return () => observer.disconnect();
}
