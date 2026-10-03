import { useEffect } from 'react';
import { describe, expect, it } from 'vitest';
import { FormProvider, useForm, type UseFormReturn } from 'react-hook-form';
import { render, act } from '@testing-library/react';
import { Form } from 'antd';
import TuicFields from '@/pages/inbounds/form/protocols/tuic';
import { rawInboundToFormValues, formValuesToWirePayload } from '@/lib/xray/inbound-form-adapter';
import { InboundFormSchema, type InboundFormValues } from '@/schemas/forms/inbound-form';
import { genTuicLink } from '@/lib/xray/inbound-link';

const legacy = {
  certificate: '/old/cert.pem',
  private_key: '/old/key.pem',
  congestion_control: 'cubic',
  log_level: 'error',
  udp_relay_mode: 'quic',
  sni: 'old.example',
  zero_rtt_handshake: false,
  max_idle_time: 77,
  authentication_timeout: 11,
  max_udp_relay_packet_size: 8192,
};
const row = { protocol: 'tuic', port: 8443, settings: legacy };
let methods: UseFormReturn<InboundFormValues>;
function Harness() {
  const form = useForm<InboundFormValues>({ defaultValues: rawInboundToFormValues(row) as never });
  useEffect(() => {
    methods = form;
  }, [form]);
  return (
    <FormProvider {...form}>
      <Form>
        <TuicFields />
      </Form>
    </FormProvider>
  );
}

describe('TUIC legacy edit and raw profile precedence', () => {
  it('mounting old flat settings must display current certificate', () => {
    render(<Harness />);
    const displayed = Array.from(document.querySelectorAll('input')).map((x) => x.value);
    expect(displayed).toContain('/old/cert.pem');
  });
  it('wraps certificate actions inside the form column on narrow layouts', () => {
    render(<Harness />);
    const actions = document.querySelector<HTMLElement>('.tuic-certificate-actions');
    expect(actions?.style.display).toBe('flex');
    expect(actions?.style.flexWrap).toBe('wrap');
    expect(actions?.style.width).toBe('100%');
    expect(actions?.querySelectorAll('button')).toHaveLength(3);
  });
  it('editing only SNI must preserve controller and runtime values', () => {
    render(<Harness />);
    act(() => methods.setValue('settings.server.sni', 'new.example'));
    const parsed = InboundFormSchema.parse(methods.getValues());
    const saved = JSON.parse(formValuesToWirePayload(parsed).settings);
    expect.soft(saved.server.congestion_control).toBe('cubic');
    expect.soft(saved.server.log_level).toBe('error');
    expect.soft(saved.server.zero_rtt_handshake).toBe(false);
    expect.soft(saved.server.max_idle_time).toBe(77);
    expect.soft(saved.server.udp_relay_mode).toBe('quic');
    expect.soft(saved.server.authentication_timeout).toBe(11);
    expect.soft(saved.server.max_udp_relay_packet_size).toBe(8192);
  });
  it('mounting then saving without TUIC changes must preserve flat values', () => {
    render(<Harness />);
    const saved = JSON.parse(
      formValuesToWirePayload(InboundFormSchema.parse(methods.getValues())).settings,
    );
    expect.soft(saved.server?.congestion_control ?? saved.congestion_control).toBe('cubic');
    expect.soft(saved.server?.sni || saved.sni).toBe('old.example');
    expect.soft(saved.server?.udp_relay_mode ?? saved.udp_relay_mode).toBe('quic');
  });
  it('empty nested controller must match runtime legacy flat fallback', () => {
    const link = genTuicLink({
      inbound: {
        protocol: 'tuic',
        port: 8443,
        settings: { ...legacy, server: { congestion_control: '' } },
      } as never,
      address: 'proxy.example',
      clientUuid: '11111111-1111-1111-1111-111111111111',
      clientPassword: 'dummy',
    });
    expect(new URL(link).searchParams.get('congestion_control')).toBe('cubic');
  });
});
