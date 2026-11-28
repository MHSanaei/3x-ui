import { useState } from 'react';
import { act, fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import NodeList from '@/pages/nodes/NodeList';
import NodeNetworkSpeed from '@/pages/nodes/NodeNetworkSpeed';
import { NodeRecordSchema } from '@/schemas/node';
import { renderWithProviders } from './test-utils';
const noop = () => {};

describe('node metric availability', () => {
  it.each([false, true])(
    'does not present unavailable CPU as measured zero (mobile=%s)',
    (isMobile) => {
      const node = NodeRecordSchema.parse({
        id: 1,
        name: 'edge',
        status: 'online',
        enable: true,
        cpuPct: 0,
        memPct: 44.4,
        uptimeSecs: 0,
        unavailableMetrics: ['cpu', 'uptime'],
      });
      renderWithProviders(
        <NodeList
          nodes={[node]}
          isMobile={isMobile}
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
          onRestart={noop}
        />,
      );
      if (isMobile) fireEvent.click(screen.getByRole('button', { name: 'More Information' }));
      expect(screen.queryByText('0.0%')).toBeNull();
      expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(2);
    },
  );
  it('keeps a measured zero CPU and an unprobed uptime apart', () => {
    const node = NodeRecordSchema.parse({
      id: 1,
      name: 'edge',
      status: 'online',
      enable: true,
      cpuPct: 0,
      memPct: 44.4,
      uptimeSecs: 0,
    });
    renderWithProviders(
      <NodeList
        nodes={[node]}
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
        onRestart={noop}
      />,
    );
    expect(screen.getByText('0.0%')).not.toBeNull();
    const headers = screen.getAllByRole('columnheader').map((h) => h.textContent);
    const cells = within(screen.getAllByRole('row')[1]).getAllByRole('cell');
    expect(cells[headers.indexOf('Uptime')].textContent).toBe('-');
  });
});

describe('node network speed', () => {
  const controls = {
    selectedIds: [],
    onSelectionChange: noop,
    onAdd: noop,
    onMtls: noop,
    onEdit: noop,
    onDelete: noop,
    onProbe: noop,
    onToggleEnable: noop,
    onUpdateNode: noop,
    onUpdateSelected: noop,
    onRestart: noop,
  };
  it.each([false, true])(
    'renders current directions and preserves measured zero (mobile=%s)',
    (isMobile) => {
      const node = NodeRecordSchema.parse({
        id: 1,
        name: 'edge',
        enable: true,
        status: 'online',
        lastHeartbeat: Math.floor(Date.now() / 1000),
        netDown: 2048,
        netUp: 0,
      });
      function Fixture() {
        const [record, setRecord] = useState(node);
        return (
          <>
            <button
              type="button"
              onClick={() =>
                setRecord({ ...record, lastHeartbeat: (node.lastHeartbeat ?? 0) - 120 })
              }
            >
              Age sample
            </button>
            <NodeList {...controls} nodes={[record]} isMobile={isMobile} />
          </>
        );
      }
      renderWithProviders(<Fixture />);
      if (isMobile) fireEvent.click(screen.getByRole('button', { name: 'More Information' }));
      expect(screen.getByLabelText(/^download .*2\.00 KB\/s$/i).textContent).toContain('2.00 KB/s');
      expect(screen.getByLabelText(/^upload .*0 B\/s$/i).textContent).toContain('0 B/s');
      // A heartbeat cache update must also refresh the open mobile stats dialog.
      fireEvent.click(screen.getByText('Age sample'));
      expect(screen.getByLabelText(/^download .*—$/i).textContent).toContain('—');
    },
  );

  it('expires throughput without another heartbeat', () => {
    vi.useFakeTimers();
    try {
      const node = NodeRecordSchema.parse({
        id: 1,
        name: 'edge',
        enable: true,
        status: 'online',
        lastHeartbeat: Math.floor(Date.now() / 1000),
        netDown: 2048,
        netUp: 0,
      });
      renderWithProviders(<NodeNetworkSpeed node={node} />);
      expect(screen.getByLabelText(/^download .*2\.00 KB\/s$/i).textContent).toContain('2.00 KB/s');
      act(() => vi.advanceTimersByTime(30_000));
      expect(screen.getByLabelText(/^download .*—$/i).textContent).toContain('—');
    } finally {
      vi.useRealTimers();
    }
  });

  it('honors disabled, legacy, and transitive availability', () => {
    const base = { enable: true, status: 'online', lastHeartbeat: Math.floor(Date.now() / 1000) };
    renderWithProviders(
      <NodeList
        {...controls}
        nodes={[
          {
            ...base,
            id: 1,
            name: 'legacy',
            unavailableMetrics: ['netUp', 'netDown'],
            netDown: 0,
            netUp: 0,
          },
          { ...base, id: 2, name: 'disabled', enable: false, netDown: 123, netUp: 123 },
          { ...base, id: 3, name: 'descendant', transitive: true, netDown: 123, netUp: 123 },
        ]}
      />,
    );
    expect(screen.getAllByLabelText(/^download .*—$/i)).toHaveLength(3);
    expect(screen.getAllByLabelText(/^upload .*—$/i)).toHaveLength(3);
  });
});
