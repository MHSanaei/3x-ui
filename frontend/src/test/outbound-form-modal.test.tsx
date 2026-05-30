import { describe, it, expect } from 'vitest';

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

  it('field structure is stable for every protocol', () => {
    renderModal(null);
    const protocols = listSelectOptions('protocol');
    expect(protocols.length).toBeGreaterThan(3);
    for (const proto of protocols) {
      chooseSelectOption('protocol', proto);
      expect(fieldLabels()).toMatchSnapshot(proto);
    }
  });
});
