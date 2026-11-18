import type { ReactNode } from 'react';
import { Form } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';
import { afterEach, describe, expect, it, vi } from 'vitest';

import MtprotoFields from '@/pages/inbounds/form/protocols/mtproto';
import { MtprotoInboundSettingsSchema } from '@/schemas/protocols/inbound/mtproto';
import { HttpUtil, Msg } from '@/utils';
import { fieldLabels, renderWithProviders } from './test-utils';

afterEach(() => {
  vi.restoreAllMocks();
});

function parse(settings: Record<string, unknown>) {
  return MtprotoInboundSettingsSchema.safeParse({ clients: [], ...settings });
}

function Harness({
  children,
  settings,
}: {
  children: ReactNode;
  settings: Record<string, unknown>;
}) {
  const methods = useForm({ defaultValues: { settings } });
  return (
    <FormProvider {...methods}>
      <Form>{children}</Form>
    </FormProvider>
  );
}

function renderFields(settings: Record<string, unknown>) {
  vi.spyOn(HttpUtil, 'post').mockResolvedValue(
    new Msg(true, '', JSON.stringify({ xraySetting: { outbounds: [] } })),
  );
  renderWithProviders(
    <Harness settings={settings}>
      <MtprotoFields />
    </Harness>,
  );
}

describe('mtproto telemt backend', () => {
  it('keeps the telemt options and drops cleared ones', () => {
    const parsed = parse({
      backend: 'telemt',
      telemt: { maxTcpConnsPerUser: 8, extraToml: '[server]\nclient_mss = "tspu"\n' },
    });
    expect(parsed.success).toBe(true);
    expect(parsed.data?.backend).toBe('telemt');
    expect(parsed.data?.telemt).toEqual({
      maxTcpConnsPerUser: 8,
      extraToml: '[server]\nclient_mss = "tspu"\n',
    });
    const cleared = parse({
      backend: 'telemt',
      telemt: { maxTcpConnsPerUser: null, extraToml: '' },
    });
    expect(cleared.data?.telemt).toEqual({});
  });

  it.each([
    ['an unknown backend', { backend: 'mtproxy' }],
    ['a connection cap above 65535', { telemt: { maxTcpConnsPerUser: 65536 } }],
  ])('rejects %s', (_name, settings) => {
    expect(parse(settings).success).toBe(false);
  });

  it('shows the telemt fields instead of the mtg-only ones', () => {
    renderFields({ backend: 'telemt' });
    const labels = fieldLabels();
    expect(labels).toContain('Backend');
    expect(labels).toContain('TCP connections per client');
    expect(labels).toContain('Extra TOML');
    expect(labels).not.toContain('Public IPv4');
  });

  it('keeps the mtg fields for an mtg inbound', () => {
    renderFields({});
    const labels = fieldLabels();
    expect(labels).toContain('Backend');
    expect(labels).toContain('Public IPv4');
    expect(labels).not.toContain('TCP connections per client');
  });
});
