import { describe, it, expect } from 'vitest';

import InboundFormModal from '@/pages/inbounds/form/InboundFormModal';
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
});
