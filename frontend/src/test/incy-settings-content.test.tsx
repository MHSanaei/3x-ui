import { describe, expect, it, vi } from 'vitest';
import { fireEvent } from '@testing-library/react';

import IncySettingsContent from '@/pages/settings/IncySettingsContent';
import { AllSetting } from '@/models/setting';

import { renderWithProviders } from './test-utils';

function openTab(name: string) {
  const tab = Array.from(document.querySelectorAll('.ant-tabs-tab')).find((t) =>
    (t.textContent ?? '').includes(name),
  );
  if (!tab) throw new Error(`tab '${name}' not found`);
  fireEvent.click(tab);
}

function settingRow(title: string): HTMLElement {
  const row = Array.from(document.querySelectorAll('li,div[class*="setting"]')).find((r) =>
    (r.textContent ?? '').includes(title),
  );
  if (!row) throw new Error(`setting row '${title}' not found`);
  return row as HTMLElement;
}

function selectFor(title: string): HTMLElement {
  const select = settingRow(title).querySelector('.ant-select');
  if (!select) throw new Error(`select for '${title}' not found`);
  return select as HTMLElement;
}

function selectOption(title: string, option: string) {
  const select = selectFor(title);
  fireEvent.mouseDown(select.querySelector('.ant-select-selector') ?? select);
  const match = Array.from(document.querySelectorAll('.ant-select-item-option')).find(
    (o) => (o.textContent ?? '').trim() === option,
  );
  if (!match) throw new Error(`option '${option}' for '${title}' not found`);
  fireEvent.click(match);
}

function render(subIncy: Partial<AllSetting> = {}) {
  const updateSetting = vi.fn();
  const allSetting = Object.assign(new AllSetting(), subIncy);
  renderWithProviders(
    <IncySettingsContent
      allSetting={allSetting}
      updateSetting={updateSetting}
      isMobile={false}
      remoteSourceBadge={() => null}
    />,
  );
  return updateSetting;
}

describe('Incy app-management settings', () => {
  // Every Incy switch and enum is tri-state: "Not set" must store the empty
  // value so the panel sends no header and the subscriber's app choice wins.
  it('stores the empty value for "Not set" so no header is emitted', () => {
    const updateSetting = render({ subIncySortOrder: 'ping' });

    openTab('App');
    selectOption('Server sort order', 'Not set');

    expect(updateSetting).toHaveBeenCalledWith({ subIncySortOrder: '' });
  });

  it('sends the documented per-app literal, not the Happ on/include spelling', () => {
    const updateSetting = render();

    openTab('Privacy');
    selectOption('Per-app mode (Android)', 'On');

    expect(updateSetting).toHaveBeenCalledWith({ subIncyPerAppEnable: '1' });
  });

  it('offers bypass and proxy for the per-app mode', () => {
    const updateSetting = render();

    openTab('Privacy');
    selectOption('Per-app mode type', 'Listed apps bypass');

    expect(updateSetting).toHaveBeenCalledWith({ subIncyPerAppMode: 'bypass' });
  });
});
