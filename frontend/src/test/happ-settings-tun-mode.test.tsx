import { describe, expect, it, vi } from 'vitest';
import { fireEvent } from '@testing-library/react';

import HappSettingsContent from '@/pages/settings/HappSettingsContent';
import { AllSetting } from '@/models/setting';

import { renderWithProviders } from './test-utils';

function openTab(name: string) {
  const tab = Array.from(document.querySelectorAll('.ant-tabs-tab')).find((t) =>
    (t.textContent ?? '').includes(name),
  );
  if (!tab) throw new Error(`tab '${name}' not found`);
  fireEvent.click(tab);
}

function selectFor(title: string): HTMLElement {
  const row = Array.from(document.querySelectorAll('.ant-select')).find((s) =>
    (s.closest('li,div[class*="setting"]')?.textContent ?? '').includes(title),
  );
  if (!row) throw new Error(`select for '${title}' not found`);
  return row as HTMLElement;
}

function clickOption(text: string) {
  const option = Array.from(document.querySelectorAll('.ant-select-item-option')).find(
    (o) => (o.textContent ?? '').trim() === text,
  );
  if (!option) throw new Error(`option '${text}' not found`);
  fireEvent.click(option);
}

describe('Happ TUN Mode select', () => {
  // happ.su documents tun-mode as system|gvisor only, so the Default entry has
  // to mean "send no header", the same state a fresh panel ships with.
  it('stores the empty value for Default so no Tun-Mode header is emitted', () => {
    const updateSetting = vi.fn();
    const allSetting = new AllSetting();
    allSetting.subHappTunMode = 'gvisor';

    renderWithProviders(
      <HappSettingsContent
        allSetting={allSetting}
        updateSetting={updateSetting}
        isMobile={false}
        remoteSourceBadge={() => null}
      />,
    );

    openTab('Network');
    const select = selectFor('TUN Mode');
    fireEvent.mouseDown(select.querySelector('.ant-select-selector') ?? select);
    clickOption('Default');

    expect(updateSetting).toHaveBeenCalledWith({ subHappTunMode: '' });
  });

  it('labels the unset state Default rather than leaving the control blank', () => {
    renderWithProviders(
      <HappSettingsContent
        allSetting={new AllSetting()}
        updateSetting={vi.fn()}
        isMobile={false}
        remoteSourceBadge={() => null}
      />,
    );

    openTab('Network');
    const select = selectFor('TUN Mode');
    expect(select.querySelector('.ant-select-content')?.textContent).toBe('Default');
  });

  it('renders Auto-Detection master switch at the top and triggers update', () => {
    const updateSetting = vi.fn();
    const allSetting = new AllSetting();
    allSetting.subHappAutoDetect = false;

    renderWithProviders(
      <HappSettingsContent
        allSetting={allSetting}
        updateSetting={updateSetting}
        isMobile={false}
        remoteSourceBadge={() => null}
      />,
    );

    const switchBtn = document.querySelector('.ant-switch');
    if (!switchBtn) throw new Error('switch not found');
    fireEvent.click(switchBtn);

    expect(updateSetting).toHaveBeenCalledWith({ subHappAutoDetect: true });
  });
});
