import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';

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

  it('field structure is stable for every protocol', () => {
    renderModal();
    const protocols = listSelectOptions('protocol');
    expect(protocols.length).toBeGreaterThan(3);
    for (const proto of protocols) {
      chooseSelectOption('protocol', proto);
      expect(fieldLabels()).toMatchSnapshot(proto);
    }
  });

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
