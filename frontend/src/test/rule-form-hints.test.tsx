import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import RuleFormModal from '@/pages/xray/routing/RuleFormModal';
import { HttpUtil, Msg } from '@/utils';

import { renderWithProviders } from './test-utils';

function hintOf(field: HTMLElement): string {
  const ids = (field.getAttribute('aria-describedby') ?? '').split(' ').filter(Boolean);
  return ids.map((id) => document.getElementById(id)?.textContent).join(' ');
}

// The "comma separated" hint lived in a hover-only tooltip on the label text, so
// the routing fields reached screen readers without it (s-ui#1293).
describe('RuleFormModal field hints', () => {
  afterEach(() => vi.restoreAllMocks());

  it.each(['IP', 'Source IPs'])('describes %s with the comma hint', (label) => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(new Msg(true, '', []));
    renderWithProviders(
      <RuleFormModal
        open
        rule={null}
        inboundTags={[]}
        outboundTags={['direct']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );

    expect(hintOf(screen.getByRole('textbox', { name: label }))).toBe('Comma-separated list');
  });
});
