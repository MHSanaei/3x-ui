import { describe, it, expect } from 'vitest';
import { screen, act } from '@testing-library/react';

import InboundFormModal from '@/pages/inbounds/form/InboundFormModal';
import { DBInbound } from '@/models/dbinbound';
import {
  renderWithProviders,
  fieldLabels,
  listSelectOptions,
  chooseSelectOption,
} from './test-utils';

function renderModal() {
  return renderWithProviders(
    <InboundFormModal
      open
      mode="add"
      dbInbound={null}
      dbInbounds={[]}
      availableNodes={[]}
      onClose={() => {}}
      onSaved={() => {}}
    />,
  );
}

describe('InboundFormModal', () => {
  it('renders add mode without crashing', () => {
    renderModal();
    expect(document.querySelector('.ant-modal')).toBeTruthy();
    expect(fieldLabels().length).toBeGreaterThan(0);
  });

  it('field structure differs per protocol (not a vacuous snapshot loop)', async () => {
    renderModal();
    const protocols = listSelectOptions('protocol');
    expect(protocols.length).toBeGreaterThan(3);

    const labelsByProto: Record<string, string[]> = {};
    for (const proto of protocols) {
      chooseSelectOption('protocol', proto);
      // Flush antd Form.useWatch('protocol') before reading — without it every iteration
      // sees the same pre-update DOM and the loop asserts nothing (the original bug here).
      await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
      labelsByProto[proto] = fieldLabels();
    }

    // The loop must actually exercise protocol-specific rendering: distinct protocols
    // must yield distinct field sets (a vacuous loop makes them all identical).
    const distinctShapes = new Set(Object.values(labelsByProto).map((l) => l.join('|')));
    expect(distinctShapes.size).toBeGreaterThan(1);

    // Spot-check a protocol-distinguishing field that must appear after the switch.
    if (labelsByProto.shadowsocks) {
      expect(labelsByProto.shadowsocks).toContain('Encryption method');
    }
  }, 30000); // iterates every protocol, re-rendering a heavy modal each time — slow on CI runners

  it('preserves custom share address strategy when editing a local inbound', async () => {
    renderWithProviders(
      <InboundFormModal
        open
        mode="edit"
        dbInbound={new DBInbound({
          id: 1,
          port: 12345,
          listen: '',
          protocol: 'shadowsocks',
          remark: 'edge',
          enable: true,
          settings: {
            method: '2022-blake3-aes-128-gcm',
            password: 'server-password',
            network: 'tcp,udp',
            clients: [],
          },
          streamSettings: { network: 'tcp', security: 'none', tcpSettings: {} },
          sniffing: { enabled: false },
          nodeId: null,
          shareAddrStrategy: 'custom',
          shareAddr: 'edge.example.test',
        })}
        dbInbounds={[]}
        availableNodes={[]}
        onClose={() => {}}
        onSaved={() => {}}
      />,
    );

    const shareAddrInput = await screen.findByDisplayValue('edge.example.test');
    expect((shareAddrInput as HTMLInputElement).value).toBe('edge.example.test');
  });
});
