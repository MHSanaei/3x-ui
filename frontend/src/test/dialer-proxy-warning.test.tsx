import { Form } from 'antd';
import { screen } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';
import { afterEach, describe, expect, it, vi } from 'vitest';

import HostSockoptForm from '@/pages/hosts/json-forms/HostSockoptForm';
import { SockoptForm } from '@/pages/xray/outbounds/transport';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

afterEach(() => {
  vi.restoreAllMocks();
});

const WARNING = 'No outbound with this tag is available to chain through.';

function Harness({ dialerProxy, outboundTags }: { dialerProxy: string; outboundTags?: string[] }) {
  const methods = useForm({ defaultValues: { streamSettings: { sockopt: { dialerProxy } } } });
  return (
    <FormProvider {...methods}>
      <Form>
        <SockoptForm outboundTags={outboundTags} />
      </Form>
    </FormProvider>
  );
}

// A Dialer Proxy that names a tag no outbound has chains to nothing, and the
// select used to keep showing such a value as if it were a normal choice.
describe('Dialer Proxy warning', () => {
  it('warns when the tag is not among the available outbounds', () => {
    renderWithProviders(<Harness dialerProxy="gone" outboundTags={['direct', 'warp']} />);
    expect(screen.getByText(WARNING)).toBeTruthy();
  });

  it('stays quiet for an available tag', () => {
    renderWithProviders(<Harness dialerProxy="warp" outboundTags={['direct', 'warp']} />);
    expect(screen.queryByText(WARNING)).toBeNull();
  });

  it('stays quiet when no tag is set', () => {
    renderWithProviders(<Harness dialerProxy="" outboundTags={['direct', 'warp']} />);
    expect(screen.queryByText(WARNING)).toBeNull();
  });

  it('stays quiet while the available tags are not known yet', () => {
    renderWithProviders(<Harness dialerProxy="gone" />);
    expect(screen.queryByText(WARNING)).toBeNull();
  });
});

// The host form loads the tags asynchronously, so an empty list while loading
// must not read as "no outbound exists".
describe('Dialer Proxy warning in the host form', () => {
  it('warns only once the outbound tags have loaded', async () => {
    const payload = {
      xraySetting: {
        outbounds: [
          { tag: 'direct', protocol: 'freedom' },
          { tag: 'warp', protocol: 'wireguard' },
        ],
      },
    };
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', JSON.stringify(payload)));

    renderWithProviders(<HostSockoptForm value='{"dialerProxy":"gone"}' />);

    expect(screen.queryByText(WARNING)).toBeNull();
    expect(await screen.findByText(WARNING)).toBeTruthy();
  });
});
