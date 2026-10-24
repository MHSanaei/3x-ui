import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import NodeList from '@/pages/nodes/NodeList';
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
      />,
    );
    expect(screen.getByText('0.0%')).not.toBeNull();
    const headers = screen.getAllByRole('columnheader').map((h) => h.textContent);
    const cells = within(screen.getAllByRole('row')[1]).getAllByRole('cell');
    expect(cells[headers.indexOf('Uptime')].textContent).toBe('-');
  });
});
