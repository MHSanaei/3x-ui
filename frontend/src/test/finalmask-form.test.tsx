import { describe, expect, it } from 'vitest';

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
