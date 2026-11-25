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

function Harness({
  dialerProxy,
  outboundTags,
  warn = true,
}: {
  dialerProxy: string;
  outboundTags?: string[];
  warn?: boolean;
}) {
  const methods = useForm({ defaultValues: { streamSettings: { sockopt: { dialerProxy } } } });
  return (
    <FormProvider {...methods}>
      <Form>
        <SockoptForm outboundTags={outboundTags} warnMissingDialerProxy={warn} />
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

// A host's Dialer Proxy resolves inside the subscriber's client config, not the
// server's outbounds, so the server-side list must never produce a verdict there.
describe('Dialer Proxy warning in the host form', () => {
  it('does not warn against the server outbound list', async () => {
    const payload = { xraySetting: { outbounds: [{ tag: 'direct', protocol: 'freedom' }] } };
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', JSON.stringify(payload)));

    renderWithProviders(<HostSockoptForm value='{"dialerProxy":"gone"}' />);

    await screen.findByText('direct', {}, { timeout: 500 }).catch(() => undefined);
    expect(screen.queryByText(WARNING)).toBeNull();
  });

  it('does not warn when the caller does not opt in', () => {
    renderWithProviders(<Harness dialerProxy="gone" outboundTags={['direct']} warn={false} />);
    expect(screen.queryByText(WARNING)).toBeNull();
  });
});
