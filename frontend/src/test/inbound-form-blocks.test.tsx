import { describe, it, expect } from 'vitest';
import { fireEvent } from '@testing-library/react';
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

  it('RealityForm shows certificate details and a local-network warning for a private target', () => {
    const scanArgs: (boolean | undefined)[] = [];
    renderInForm(
      <RealityForm
        saving={false}
        scanning={false}
        scanResult={{
          target: 'nginx:443',
          host: 'nginx',
          ip: '',
          port: 443,
          feasible: false,
          privateTarget: true,
          tls13: true,
          tlsVersion: '1.3',
          h2: true,
          alpn: 'h2',
          x25519: true,
          curveID: 'X25519',
          certValid: false,
          certSubject: 'front.example.com',
          certIssuer: 'Acme Co',
          notAfter: '2026-08-01T00:00:00Z',
          serverNames: ['front.example.com'],
          latencyMs: 3,
          reason: 'certificate not trusted: x509: certificate signed by unknown authority',
        }}
        scanRealityTarget={(allowPrivate) => scanArgs.push(allowPrivate)}
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
    const alert = document.querySelector('.ant-alert');
    expect(alert?.textContent).toContain('front.example.com (Acme Co)');
    expect(alert?.textContent).toContain('Not trusted');
    expect(alert?.textContent).toContain('private/local network');
    expect(alert?.className).toContain('ant-alert-warning');

    // The click event must not reach the allowPrivate argument, or a plain
    // "Check" click would silently opt the probe out of the SSRF guard.
    const scanButton = Array.from(document.querySelectorAll('button'))
      .find((b) => (b.textContent ?? '').trim() === 'Scan');
    if (!scanButton) throw new Error('scan button not rendered');
    fireEvent.click(scanButton);
    expect(scanArgs).toEqual([undefined]);
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
