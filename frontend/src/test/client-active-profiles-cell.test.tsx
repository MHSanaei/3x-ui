import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ClientActiveProfiles } from '@/pages/clients/RowCells';

describe('active profile cell', () => {
  const inbounds = {
    1: { id: 1, tag: 'vless', remark: 'VLESS home', protocol: 'vless', originNodeGuid: 'a' },
    2: { id: 2, tag: 'hy', remark: 'Hysteria home', protocol: 'hysteria', originNodeGuid: 'a' },
  };
  const props = {
    email: 'alice',
    ids: [1, 2],
    inboundsById: inbounds,
    protocolColors: { vless: 'blue', hysteria: 'purple' },
  };
  it('shows all live profile names and removes closed ones on the next snapshot', () => {
    const { rerender } = render(
      <ClientActiveProfiles
        {...props}
        tree={{ a: { supported: true, observedAt: 1, clients: { alice: ['vless', 'hy'] } } }}
      />,
    );
    expect(screen.getByText('VLESS home')).toBeTruthy();
    expect(screen.getByText('Hysteria home')).toBeTruthy();
    rerender(
      <ClientActiveProfiles
        {...props}
        tree={{ a: { supported: true, observedAt: 2, clients: { alice: ['hy'] } } }}
      />,
    );
    expect(screen.queryByText('VLESS home')).toBeNull();
    expect(screen.getByText('Hysteria home')).toBeTruthy();
    rerender(
      <ClientActiveProfiles
        {...props}
        tree={{ a: { supported: true, observedAt: 3, clients: {} } }}
      />,
    );
    expect(screen.queryByText('Hysteria home')).toBeNull();
  });
});
