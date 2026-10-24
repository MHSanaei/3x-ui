import { useState } from 'react';
import { fireEvent, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import RoutingTab from '@/pages/xray/routing/RoutingTab';
import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { renderWithProviders } from './test-utils';
import { installPointerEvents, pointAt } from './pointer-events';

const initial = {
  routing: {
    domainStrategy: 'AsIs',
    rules: [
      { type: 'field', outboundTag: 'a', enabled: true },
      { type: 'field', inboundTag: ['_bl_bal1'], outboundTag: 'hidden', enabled: true },
      { type: 'field', outboundTag: 'b', enabled: true },
      { type: 'field', outboundTag: 'c', enabled: true },
    ],
  },
} as unknown as XraySettingsValue;

function Harness({ mobile }: { mobile: boolean }) {
  const [settings, setSettings] = useState<XraySettingsValue | null>(initial);
  return (
    <>
      <RoutingTab
        templateSettings={settings}
        setTemplateSettings={setSettings}
        inboundTags={[]}
        clientReverseTags={[]}
        isMobile={mobile}
      />
      <output data-testid="settings">{JSON.stringify(settings)}</output>
    </>
  );
}

function ruleOrder() {
  const settings = JSON.parse(
    screen.getByTestId('settings').textContent || '{}',
  ) as XraySettingsValue;
  return settings.routing?.rules?.map((rule) => rule.outboundTag);
}

function openRouting(mobile: boolean) {
  const view = renderWithProviders(<Harness mobile={mobile} />);
  fireEvent.click(screen.getByRole('tab', { name: /Routing Rules/ }));
  return {
    ...view,
    rows: () =>
      document.querySelectorAll(mobile ? '.rule-card' : '.routing-table tr[data-row-key]'),
  };
}

function gesture(
  rows: NodeListOf<Element>,
  from: number,
  to: number,
  cancel = false,
  touch = false,
) {
  pointAt(rows[to]);
  const event = { pointerId: 7, button: 0, pointerType: touch ? 'touch' : 'mouse' };
  fireEvent.pointerDown(rows[from].querySelector('.drag-handle')!, { ...event, clientY: 0 });
  fireEvent.pointerMove(document, { ...event, clientY: 32, clientX: 10 });
  fireEvent[cancel ? 'pointerCancel' : 'pointerUp'](document, {
    ...event,
    clientY: 32,
    clientX: 10,
  });
}

describe.each([false, true])('Routing reorder (mobile=%s)', (mobile) => {
  beforeEach(installPointerEvents);
  afterEach(() => vi.unstubAllGlobals());

  it('maps visible positions through hidden loopback rules when moving upward', () => {
    const view = openRouting(mobile);
    expect(view.rows()).toHaveLength(3);
    gesture(view.rows(), 1, 0, false, mobile);
    expect(ruleOrder()).toEqual(['b', 'a', 'hidden', 'c']);
  });

  it('retains the old splice destination when moving downward', () => {
    const view = openRouting(mobile);
    gesture(view.rows(), 0, 2, false, mobile);
    expect(ruleOrder()).toEqual(['hidden', 'b', 'c', 'a']);
  });

  it('maps a drop target after a hidden loopback back to the intended rule', () => {
    const view = openRouting(mobile);
    gesture(view.rows(), 0, 1, false, mobile);
    expect(ruleOrder()).toEqual(['hidden', 'b', 'a', 'c']);
  });

  it('never moves rules when the active pointer is cancelled', () => {
    const view = openRouting(mobile);
    gesture(view.rows(), 1, 0, true, mobile);
    expect(ruleOrder()).toEqual(['a', 'hidden', 'b', 'c']);
  });
});
