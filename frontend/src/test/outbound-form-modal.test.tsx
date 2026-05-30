import { describe, it, expect } from 'vitest';

import OutboundFormModal from '@/pages/xray/outbounds/OutboundFormModal';
import { renderWithProviders, fieldLabels } from './test-utils';

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

  it('add-mode field structure is stable', () => {
    renderModal(null);
    expect(fieldLabels()).toMatchSnapshot();
  });
});
