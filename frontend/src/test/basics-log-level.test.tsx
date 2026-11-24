import { describe, it, expect, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';

import BasicsTab from '@/pages/xray/basics/BasicsTab';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

import { renderWithProviders } from './test-utils';

// xray-core 26.11.9 (#7329) spells the key log.logLevel and still reads loglevel.
describe('BasicsTab log level spelling', () => {
  it('shows a template written as logLevel and saves one key, not both spellings', () => {
    const setTemplateSettings = vi.fn();
    renderWithProviders(
      <BasicsTab
        templateSettings={{ log: { logLevel: 'debug' } } as unknown as XraySettingsValue}
        setTemplateSettings={setTemplateSettings}
        outboundTestUrl=""
        onChangeOutboundTestUrl={vi.fn()}
        onResetDefault={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole('tab', { name: /Log/ }));
    const combobox = screen.getByRole('combobox', { name: 'Log Level' });
    expect(combobox.closest('.ant-select')?.textContent).toContain('debug');

    fireEvent.mouseDown(combobox);
    fireEvent.click(screen.getByTitle('error'));
    const update = setTemplateSettings.mock.calls.at(-1)?.[0] as (
      prev: XraySettingsValue,
    ) => XraySettingsValue;
    const legacy = { log: { loglevel: 'warning' } } as unknown as XraySettingsValue;
    expect(update(legacy).log).toEqual({ logLevel: 'error' });
  });
});
