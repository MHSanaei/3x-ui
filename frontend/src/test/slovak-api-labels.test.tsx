import { afterEach, describe, expect, it, vi } from 'vitest';
import { waitFor } from '@testing-library/react';
import { createInstance } from 'i18next';

import enUS from '../../../internal/web/translation/en-US.json';
import skSK from '../../../internal/web/translation/sk-SK.json';
import { bindSwaggerLabels } from '@/pages/api-docs/swagger-labels';

let stop: (() => void) | undefined;
afterEach(() => {
  stop?.();
  document.body.replaceChildren();
});

async function translator() {
  const i18n = createInstance();
  await i18n.init({
    lng: 'sk-SK',
    resources: { 'sk-SK': { translation: skSK }, 'en-US': { translation: enUS } },
  });
  return i18n.t;
}

describe('Swagger display labels', () => {
  it('keeps live buttons, input values and technical examples intact', async () => {
    const root = document.createElement('div');
    root.innerHTML =
      '<button title="Execute">Execute</button><h4>Responses</h4><pre><code>Execute</code></pre><input value="Execute"><span class="parameter__name">Name</span>';
    document.body.append(root);
    const button = root.querySelector('button')!;
    const originalText = button.firstChild;
    const handler = vi.fn();
    button.addEventListener('click', handler);
    stop = bindSwaggerLabels(root, await translator());
    expect(button.textContent).toBe('Odoslať požiadavku');
    expect(button.title).toBe('Odoslať požiadavku');
    expect(button.firstChild).toBe(originalText);
    button.click();
    expect(handler).toHaveBeenCalledOnce();
    expect(root.querySelector('h4')?.textContent).toBe('Odpovede');
    expect(root.querySelector('code')?.textContent).toBe('Execute');
    expect(root.querySelector('input')?.value).toBe('Execute');
    expect(root.querySelector('.parameter__name')?.textContent).toBe('Name');
  });

  it('handles later Swagger renders and releases the observer on unmount', async () => {
    const root = document.createElement('div');
    document.body.append(root);
    stop = bindSwaggerLabels(root, await translator());
    const button = document.createElement('button');
    button.textContent = 'Cancel';
    root.append(button);
    await waitFor(() => expect(button.textContent).toBe('Zrušiť'));
    button.textContent = 'Try it out';
    await waitFor(() => expect(button.textContent).toBe('Vyskúšať'));
    stop();
    button.textContent = 'Cancel';
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(button.textContent).toBe('Cancel');
  });
});
