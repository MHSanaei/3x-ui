import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import { FinalMaskField } from '@/lib/xray/forms/fields';

import { listSelectOptions, renderWithProviders } from './test-utils';

describe('FinalMaskForm item types', () => {
  // Only the noise mask parses tag expressions; header-custom items go through the
  // core's PraseByteSlice, which refuses "exp" and fails the whole config load.
  it('offers the exp type to noise items only', () => {
    renderWithProviders(
      <FinalMaskField
        network="kcp"
        protocol="vless"
        value={{
          tcp: [],
          udp: [
            { type: 'noise', settings: { noise: [{ type: 'array', rand: '1-8', delay: '1-2' }] } },
            { type: 'header-custom', settings: { client: [{ type: 'array', rand: 1 }] } },
          ],
        }}
      />,
    );

    // An opened dropdown stays in the DOM, so read header-custom's before noise's.
    expect(listSelectOptions('finalmask_udp_1_settings_client_0_type')).not.toContain('Expression');
    expect(listSelectOptions('finalmask_udp_0_settings_noise_0_type')).toContain('Expression');
  });
});

describe('FinalMaskForm filter mask', () => {
  // xray-core 26.10.21's filter screens a listener's source IPs and is a no-op on a dialer, so
  // only the inbound editor offers it; an older client core refuses the unknown type.
  it('offers filter on the server side only', () => {
    const value = { tcp: [{ type: 'fragment' as const, settings: {} }], udp: [] };
    const client = renderWithProviders(
      <FinalMaskField network="tcp" protocol="vless" side="client" value={value} />,
    );
    expect(listSelectOptions('finalmask_tcp_0_type')).not.toContain('Filter (source IPs)');
    client.unmount();

    renderWithProviders(
      <FinalMaskField network="tcp" protocol="vless" side="server" value={value} />,
    );
    expect(listSelectOptions('finalmask_tcp_0_type')).toContain('Filter (source IPs)');
  });
});

describe('FinalMaskForm salamander password', () => {
  // xray-core loads a salamander password under 4 bytes and refuses it only when the socket
  // opens: on an inbound that stops Xray. It counts bytes, so "a€" (4 bytes) is long enough.
  it('flags a password under 4 bytes and accepts a 4-byte one', async () => {
    renderWithProviders(
      <FinalMaskField
        network="hysteria"
        protocol="hysteria"
        value={{ tcp: [], udp: [{ type: 'salamander', settings: { password: 'long-enough' } }] }}
      />,
    );
    const input = document.getElementById('finalmask_udp_0_settings_password') as HTMLInputElement;

    fireEvent.change(input, { target: { value: 'abc' } });
    expect(await screen.findByText(/at least 4 bytes/)).toBeTruthy();

    fireEvent.change(input, { target: { value: 'a€' } });
    await waitFor(() => expect(screen.queryByText(/at least 4 bytes/)).toBeNull());
  });
});
