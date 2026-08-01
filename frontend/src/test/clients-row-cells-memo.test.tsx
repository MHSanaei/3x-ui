import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ClientInboundChips, ClientRowActions } from '@/pages/clients/RowCells';
import type { InboundOption } from '@/hooks/useClients';

const PROTOCOL_COLORS = { vless: 'blue', trojan: 'volcano' };

// Counts how often the cell reads the inbound map, which happens once per chip
// per render. A traffic push re-renders the row, so if the cell is not memoised
// this climbs every five seconds for every visible row.
function countingInboundMap(source: Record<number, InboundOption>) {
  const reads = { count: 0 };
  const proxy = new Proxy(source, {
    get(target, key) {
      if (typeof key === 'string' && /^\d+$/.test(key)) reads.count += 1;
      return target[key as unknown as number];
    },
  });
  return { proxy, reads };
}

const INBOUNDS: Record<number, InboundOption> = {
  1: { id: 1, tag: 'in-vless', remark: 'DE', protocol: 'vless' },
  2: { id: 2, tag: 'in-trojan', remark: 'NL', protocol: 'trojan' },
};

function Harness({ children }: { children: (bump: () => void) => React.ReactNode }) {
  const [, setTick] = useState(0);
  return <>{children(() => setTick((n) => n + 1))}</>;
}

describe('clients table row cells', () => {
  it('does not re-render the inbound chips when the row re-renders with the same attachments', async () => {
    const { proxy, reads } = countingInboundMap(INBOUNDS);
    const ids = [1, 2];
    let bump: () => void = () => {};

    render(
      <Harness>
        {(doBump) => {
          bump = doBump;
          return (
            <ClientInboundChips ids={ids} inboundsById={proxy} protocolColors={PROTOCOL_COLORS} chipLimit={1} />
          );
        }}
      </Harness>,
    );

    const afterFirstRender = reads.count;
    expect(afterFirstRender).toBeGreaterThan(0);

    // Three simulated traffic pushes: the parent re-renders, the props do not change.
    for (let i = 0; i < 3; i++) bump();
    await Promise.resolve();

    expect(reads.count).toBe(afterFirstRender);
  });

  it('re-renders the chips when the attachments actually change', async () => {
    const { proxy, reads } = countingInboundMap(INBOUNDS);

    function Swapper() {
      const [ids, setIds] = useState<number[]>([1]);
      return (
        <>
          <button type="button" onClick={() => setIds([1, 2])}>swap</button>
          <ClientInboundChips ids={ids} inboundsById={proxy} protocolColors={PROTOCOL_COLORS} chipLimit={1} />
        </>
      );
    }
    render(<Swapper />);
    const before = reads.count;
    await userEvent.click(screen.getByRole('button', { name: 'swap' }));
    expect(reads.count).toBeGreaterThan(before);
  });

  it('keeps the row actions wired to the right client across re-renders', async () => {
    const onShowQr = vi.fn();
    const onEdit = vi.fn();
    const noop = vi.fn();
    let bump: () => void = () => {};

    render(
      <Harness>
        {(doBump) => {
          bump = doBump;
          return (
            <ClientRowActions
              email="alice@x"
              onShowQr={onShowQr}
              onShowInfo={noop}
              onResetTraffic={noop}
              onEdit={onEdit}
              onDelete={noop}
            />
          );
        }}
      </Harness>,
    );

    for (let i = 0; i < 3; i++) bump();

    // Queried by position rather than label: the suite loads the real en-US
    // bundle, so the aria-labels are translated strings, not keys. Order is
    // QR, info, reset traffic, edit, delete.
    const buttons = screen.getAllByRole('button');
    expect(buttons).toHaveLength(5);
    await userEvent.click(buttons[0]);
    await userEvent.click(buttons[3]);

    expect(onShowQr).toHaveBeenCalledExactlyOnceWith('alice@x');
    expect(onEdit).toHaveBeenCalledExactlyOnceWith('alice@x');
  });
});
