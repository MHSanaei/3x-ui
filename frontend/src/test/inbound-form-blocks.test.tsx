import { describe, it, expect } from 'vitest';
import { Form } from 'antd';
import type { ReactNode } from 'react';
import { FormProvider, useForm } from 'react-hook-form';

import {
  GrpcForm,
  HttpUpgradeForm,
  KcpForm,
  RawForm,
  SockoptForm,
  WsForm,
  XhttpForm,
} from '@/pages/inbounds/form/transport';
import { RealityForm, TlsForm } from '@/pages/inbounds/form/security';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';
import { renderWithProviders, fieldLabels } from './test-utils';

function FormHarness({
  children,
  defaultValues,
}: {
  children: ReactNode;
  defaultValues?: Record<string, unknown>;
}) {
  const methods = useForm<InboundFormValues>({ defaultValues: defaultValues as never });
  return (
    <FormProvider {...methods}>
      <Form>{children}</Form>
    </FormProvider>
  );
}

function renderInForm(node: ReactNode, defaultValues?: Record<string, unknown>) {
  return renderWithProviders(<FormHarness defaultValues={defaultValues}>{node}</FormHarness>);
}

const noop = () => {};

describe('inbound transport forms', () => {
  it('RawForm field structure is stable', () => {
    renderInForm(<RawForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('WsForm field structure is stable', () => {
    renderInForm(<WsForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('GrpcForm field structure is stable', () => {
    renderInForm(<GrpcForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('KcpForm field structure is stable', () => {
    renderInForm(<KcpForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('HttpUpgradeForm field structure is stable', () => {
    renderInForm(<HttpUpgradeForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('XhttpForm field structure is stable', () => {
    renderInForm(<XhttpForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('SockoptForm field structure is stable (server-side fields only)', () => {
    /* The inbound sockopt form shows only server/listening-side fields;
       outbound-only fields (dialerProxy, domainStrategy, interface,
       addressPortStrategy, happyEyeballs, tcpMptcp) live in the outbound form. */
    renderInForm(
      <SockoptForm toggleSockopt={noop} network="tcp" />,
      { streamSettings: { sockopt: { mark: 0 } } },
    );
    expect(fieldLabels()).toMatchSnapshot();
  });
});

describe('inbound security forms', () => {
  it('TlsForm field structure is stable', () => {
    renderInForm(
      <TlsForm
        saving={false}
        setCertFromPanel={noop}
        clearCertFiles={noop}
        pinFromCert={noop}
        pinFromRemote={noop}
        getNewEchCert={noop}
        clearEchCert={noop}
      />,
    );
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('RealityForm field structure is stable', () => {
    renderInForm(
      <RealityForm
        saving={false}
        scanning={false}
        scanResult={null}
        scanRealityTarget={noop}
        scanRealityCandidates={async () => []}
        applyRealityScanResult={noop}
        randomizeShortIds={noop}
        randomizeSpiderX={noop}
        genRealityKeypair={noop}
        clearRealityKeypair={noop}
        genMldsa65={noop}
        clearMldsa65={noop}
      />,
    );
    expect(fieldLabels()).toMatchSnapshot();
  });
});
