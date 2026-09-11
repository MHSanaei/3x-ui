import { describe, it, expect } from 'vitest';
import { Form } from 'antd';
import type { ReactNode } from 'react';
import { FormProvider, useForm } from 'react-hook-form';

import { RealityForm } from '@/pages/inbounds/form/security';
import RealityTargetScannerModal from '@/pages/inbounds/form/security/RealityTargetScannerModal';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';
import type { RealityScanResult } from '@/generated/types';
import { renderWithProviders } from './test-utils';

const smallChain: RealityScanResult = {
  alpn: 'h2',
  certChainBytes: 3427,
  certChainValid: true,
  certIssuer: 'Google Trust Services',
  certSubject: 'cloudflare.com',
  certValid: true,
  curveID: 'X25519',
  feasible: true,
  h2: true,
  host: 'www.cloudflare.com',
  ip: '104.16.124.96',
  latencyMs: 180,
  notAfter: '2026-08-01T00:00:00Z',
  port: 443,
  privateTarget: false,
  reason: '',
  serverNames: ['www.cloudflare.com'],
  target: 'www.cloudflare.com:443',
  tls13: true,
  tlsVersion: '1.3',
  x25519: true,
};

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

const noop = () => {};

function renderRealityForm(scanResult: RealityScanResult | null, defaultValues?: Record<string, unknown>) {
  return renderWithProviders(
    <FormHarness defaultValues={defaultValues}>
      <RealityForm
        saving={false}
        scanning={false}
        scanResult={scanResult}
        scanRealityTarget={noop}
        scanRealityCandidates={async () => []}
        applyRealityScanResult={noop}
        randomizeShortIds={noop}
        randomizeSpiderX={noop}
        genRealityKeypair={noop}
        clearRealityKeypair={noop}
        genMldsa65={noop}
        clearMldsa65={noop}
      />
    </FormHarness>,
  );
}

describe('ML-DSA-65 cert chain warning', () => {
  it('warns on the inbound form when ML-DSA-65 is on and the scanned chain is under 3500 bytes', () => {
    const { getByText } = renderRealityForm(smallChain, {
      streamSettings: { realitySettings: { mldsa65Seed: 'seed' } },
    });
    expect(getByText(/below the 3500-byte minimum required for ML-DSA-65/)).toBeTruthy();
  });

  it('does not warn on the inbound form when ML-DSA-65 is off', () => {
    const { queryByText } = renderRealityForm(smallChain);
    expect(queryByText(/below the 3500-byte minimum required for ML-DSA-65/)).toBeNull();
  });

  it('tags scanner rows whose cert chain is too small for ML-DSA-65', async () => {
    const { findByText } = renderWithProviders(
      <RealityTargetScannerModal
        open
        onClose={noop}
        scanRealityCandidates={async () => [smallChain]}
        onPick={noop}
      />,
    );
    expect(await findByText('3427 B')).toBeTruthy();
  });
});
