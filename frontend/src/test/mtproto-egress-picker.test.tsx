import type { ReactNode } from 'react';
import { Form } from 'antd';
import { waitFor } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';
import { afterEach, describe, expect, it, vi } from 'vitest';

import MtprotoFields from '@/pages/inbounds/form/protocols/mtproto';
import { HttpUtil, Msg } from '@/utils';
import { listSelectOptions, renderWithProviders } from './test-utils';

afterEach(() => {
  vi.restoreAllMocks();
});

// The picker exists so Telegram traffic can be sent through a proxy; offering
// the block outbound there looks like a working selection and drops the traffic.
function mockConfigWithBlockOutbound() {
  const payload = {
    xraySetting: {
      outbounds: [
        { tag: 'direct', protocol: 'freedom' },
        { tag: 'blocked', protocol: 'blackhole' },
        { tag: 'warp', protocol: 'wireguard' },
      ],
    },
  };
  vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', JSON.stringify(payload)));
}

function Harness({ children }: { children: ReactNode }) {
  const methods = useForm({ defaultValues: { settings: { routeThroughXray: true } } });
  return (
    <FormProvider {...methods}>
      <Form>{children}</Form>
    </FormProvider>
  );
}

// The field carries an explicit id (like the inbound form's protocol select), so
// the assertions can't drift onto another select that happens to be nearby.
const EGRESS_FIELD = 'mtprotoOutboundTag';

describe('mtproto egress picker', () => {
  it('offers the routable tags and not the block outbound', async () => {
    mockConfigWithBlockOutbound();
    renderWithProviders(
      <Harness>
        <MtprotoFields />
      </Harness>,
    );

    await waitFor(() => expect(listSelectOptions(EGRESS_FIELD)).toContain('direct'));

    const options = listSelectOptions(EGRESS_FIELD);
    expect(options).toContain('warp');
    expect(options).not.toContain('blocked');
  });
});
