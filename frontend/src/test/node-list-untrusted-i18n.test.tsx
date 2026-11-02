import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import i18next from 'i18next';

import NodeList from '@/pages/nodes/NodeList';
import type { NodeRecord } from '@/schemas/node';

import { renderWithProviders } from './test-utils';

const noop = () => {};

// A closed reference: an unclosed "$t(" would hang a regressed run instead of failing it.
const reported = '$t(pages.nodes.subNode)';

function renderSubNode(fields: Partial<NodeRecord>) {
  const nodes: NodeRecord[] = [
    { id: 1, name: 'parent', guid: 'p1', enable: true, status: 'online' },
    // parentGuid is its own GUID: the sub-node tries to put its name into the parent tooltip.
    { id: 0, name: 'child', guid: 'c1', parentGuid: 'c1', transitive: true, ...fields },
  ];
  renderWithProviders(
    <NodeList
      nodes={nodes}
      isMobile={false}
      selectedIds={[]}
      onSelectionChange={noop}
      onAdd={noop}
      onMtls={noop}
      onEdit={noop}
      onDelete={noop}
      onProbe={noop}
      onToggleEnable={noop}
      onUpdateNode={noop}
      onUpdateSelected={noop}
    />,
  );
}

describe('NodeList sub-node rows', () => {
  afterEach(() => vi.restoreAllMocks());

  it('shows a node-reported status it does not know as Unknown, not as an i18next key', () => {
    renderSubNode({ status: reported });

    const row = screen.getByText('child').closest('tr') as HTMLElement;
    expect(row.textContent).toContain('Unknown');
    expect(row.textContent).not.toContain('statusValues');
  });

  it('never names a sub-node as the parent it is reached through', () => {
    const translate = vi.spyOn(i18next, 't');
    renderSubNode({ name: reported, status: 'online' });

    expect(translate).toHaveBeenCalledWith(
      'pages.nodes.subNodeTip',
      expect.objectContaining({ parent: '-' }),
    );
    expect(JSON.stringify(translate.mock.calls)).not.toContain(reported);
  });
});

describe('i18next nesting', () => {
  afterEach(() => vi.restoreAllMocks());

  // Closed references only: an unclosed "$t(" would hang a regressed run instead of failing it.
  it('is off, so "$t(...)" in a missing key or an interpolated value stays plain text', async () => {
    const utils = await import('@/utils');
    vi.spyOn(utils.LanguageManager, 'getLanguage').mockReturnValue('en-US');
    const { readyI18n } = await import('@/i18n/react');
    const i18n = await readyI18n();

    expect(i18n.t('$t(pages.nodes.subNode)')).toBe('$t(pages.nodes.subNode)');
    expect(i18n.t('pages.nodes.subNodeTip', { parent: '$t(pages.nodes.subNode)' })).toBe(
      "Read-only: a downstream node reached through $t(pages.nodes.subNode). Manage it from $t(pages.nodes.subNode)'s own panel.",
    );
  });
});
