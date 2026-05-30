import { describe, it, expect } from 'vitest';
import { Form, type FormInstance } from 'antd';
import type { ReactNode } from 'react';

import {
  GrpcForm,
  HttpUpgradeForm,
  KcpForm,
  RawForm,
  WsForm,
  XhttpForm,
} from '@/pages/inbounds/form/transport';
import { RealityForm, TlsForm } from '@/pages/inbounds/form/security';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';
import { renderWithProviders, fieldLabels } from './test-utils';

function FormHarness({ children }: { children: (form: FormInstance<InboundFormValues>) => ReactNode }) {
  const [form] = Form.useForm<InboundFormValues>();
  return <Form form={form}>{children(form)}</Form>;
}

function renderInForm(node: (form: FormInstance<InboundFormValues>) => ReactNode) {
  return renderWithProviders(<FormHarness>{node}</FormHarness>);
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
