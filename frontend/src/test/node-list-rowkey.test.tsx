import { describe, it, expect, vi, afterEach } from 'vitest';

import NodeList from '@/pages/nodes/NodeList';
import type { NodeRecord } from '@/schemas/node';

import { renderWithProviders } from './test-utils';

const noop = () => {};

function sampleNodes(): NodeRecord[] {
  return [
    { id: 1, name: 'parent', guid: 'p1', transitive: false, enable: true, status: 'online' },
    { id: 0, name: 'child-a', guid: 'ca', parentGuid: 'p1', transitive: true },
    { id: 0, name: 'child-b', guid: 'cb', parentGuid: 'p1', transitive: true },
  ];
}

describe('NodeList desktop table row keys', () => {
  afterEach(() => vi.restoreAllMocks());

  it('gives transitive sub-node rows distinct keys instead of colliding on id 0', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    renderWithProviders(
      <NodeList
        nodes={sampleNodes()}
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

    const duplicateKeyWarning = errorSpy.mock.calls.some((call) =>
      call.some((arg) => typeof arg === 'string' && arg.includes('same key')),
    );
    expect(duplicateKeyWarning).toBe(false);
  });
});
