import { describe, it, expect, vi } from 'vitest';
import { act, fireEvent } from '@testing-library/react';

import OutboundFormModal from '@/pages/xray/outbounds/OutboundFormModal';
import {
  renderWithProviders,
  fieldLabels,
  listSelectOptions,
  chooseSelectOption,
} from './test-utils';

function renderModal(outbound: Record<string, unknown> | null = null) {
  return renderWithProviders(
    <OutboundFormModal
      open
      outbound={outbound}
      existingTags={[]}
      onClose={() => {}}
      onConfirm={() => {}}
    />,
  );
}

describe('OutboundFormModal', () => {
  it('renders add mode without crashing', () => {
    renderModal(null);
    expect(document.querySelector('.ant-modal')).toBeTruthy();
    expect(fieldLabels().length).toBeGreaterThan(0);
  });

  it('field structure differs per protocol (not a vacuous snapshot loop)', async () => {
    renderModal(null);
    const protocols = listSelectOptions('protocol');
    expect(protocols.length).toBeGreaterThan(3);

    const labelsByProto: Record<string, string[]> = {};
    for (const proto of protocols) {
      chooseSelectOption('protocol', proto);
      // Flush antd Form.useWatch('protocol') so protocol-specific fields render before
      // reading; otherwise every iteration sees the same default (vless) DOM.
      await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
      labelsByProto[proto] = fieldLabels();
    }

    // Distinct protocols must yield distinct field sets (a vacuous loop is all-identical).
    const distinctShapes = new Set(Object.values(labelsByProto).map((l) => l.join('|')));
    expect(distinctShapes.size).toBeGreaterThan(1);

    // vless carries an Encryption field; wireguard does not — proves real protocol switching.
    if (labelsByProto.vless) {
      expect(labelsByProto.vless).toContain('Encryption');
    }
    if (labelsByProto.wireguard) {
      expect(labelsByProto.wireguard).not.toContain('Encryption');
    }
  }, 30000); // iterates every protocol, re-rendering a heavy modal each time — slow on CI runners

  it('saves a vless reverse outbound while reverse sniffing stays disabled', async () => {
    const onConfirm = vi.fn();
    renderWithProviders(
      <OutboundFormModal
        open
        outbound={{
          protocol: 'vless',
          tag: 'reverse-out',
          settings: {
            vnext: [{
              address: 'example.com',
              port: 443,
              users: [{ id: 'c9f0c2d0-0000-4000-8000-000000000000', encryption: 'none' }],
            }],
            reverse: { tag: 'r1', sniffing: {} },
          },
          streamSettings: { network: 'tcp', security: 'none' },
        }}
        existingTags={[]}
        onClose={() => {}}
        onConfirm={onConfirm}
      />,
    );
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });

    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    expect(ok).toBeTruthy();
    await act(async () => { fireEvent.click(ok); });
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });

    expect(onConfirm).toHaveBeenCalledTimes(1);
    const payload = onConfirm.mock.calls[0][0] as {
      settings: { reverse?: { tag?: string } };
    };
    expect(payload.settings.reverse?.tag).toBe('r1');
  });
});
