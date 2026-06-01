import { describe, it, expect } from 'vitest';
import { Form, type FormInstance } from 'antd';
import type { ReactNode } from 'react';

import {
  ExternalProxyForm,
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
  initialValues,
}: {
  children: (form: FormInstance<InboundFormValues>) => ReactNode;
  initialValues?: Record<string, unknown>;
}) {
  const [form] = Form.useForm<InboundFormValues>();
  return <Form form={form} initialValues={initialValues}>{children(form)}</Form>;
}

function renderInForm(
  node: (form: FormInstance<InboundFormValues>) => ReactNode,
  initialValues?: Record<string, unknown>,
) {
  return renderWithProviders(<FormHarness initialValues={initialValues}>{node}</FormHarness>);
}

const noop = () => {};

describe('inbound transport forms', () => {
  it('RawForm field structure is stable', () => {
    renderInForm(() => <RawForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('WsForm field structure is stable', () => {
    renderInForm(() => <WsForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('GrpcForm field structure is stable', () => {
    renderInForm(() => <GrpcForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('KcpForm field structure is stable', () => {
    renderInForm(() => <KcpForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('HttpUpgradeForm field structure is stable', () => {
    renderInForm(() => <HttpUpgradeForm />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('XhttpForm field structure is stable', () => {
    renderInForm((form) => <XhttpForm form={form} />);
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('ExternalProxyForm field structure is stable (one TLS entry)', () => {
    renderInForm(
      () => <ExternalProxyForm toggleExternalProxy={noop} />,
      {
        streamSettings: {
          externalProxy: [{
            forceTls: 'tls',
            dest: '',
            port: 443,
            remark: '',
            sni: '',
            fingerprint: '',
            alpn: [],
          }],
        },
      },
    );
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('SockoptForm field structure is stable (enabled + happy eyeballs)', () => {
    renderInForm(
      () => <SockoptForm toggleSockopt={noop} />,
      { streamSettings: { sockopt: { happyEyeballs: {} } } },
    );
    expect(fieldLabels()).toMatchSnapshot();
  });
});

describe('inbound security forms', () => {
  it('TlsForm field structure is stable', () => {
    renderInForm(() => (
      <TlsForm
        saving={false}
        setCertFromPanel={noop}
        clearCertFiles={noop}
        generateRandomPinHash={noop}
        getNewEchCert={noop}
        clearEchCert={noop}
      />
    ));
    expect(fieldLabels()).toMatchSnapshot();
  });

  it('RealityForm field structure is stable', () => {
    renderInForm(() => (
      <RealityForm
        saving={false}
        randomizeRealityTarget={noop}
        randomizeShortIds={noop}
        genRealityKeypair={noop}
        clearRealityKeypair={noop}
        genMldsa65={noop}
        clearMldsa65={noop}
      />
    ));
    expect(fieldLabels()).toMatchSnapshot();
  });
});
