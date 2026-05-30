import { describe, it, expect } from 'vitest';

import InboundFormModal from '@/pages/inbounds/form/InboundFormModal';
import { renderWithProviders, fieldLabels } from './test-utils';

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

  it('add-mode field structure is stable', () => {
    renderModal();
    expect(fieldLabels()).toMatchSnapshot();
  });
});
