import { describe, expect, it } from 'vitest';
import { Form } from 'antd';
import type { ReactNode } from 'react';
import { FormProvider, useForm, useFormContext, useWatch } from 'react-hook-form';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';

import { TlsForm } from '@/pages/inbounds/form/security';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';
import { renderWithProviders } from './test-utils';

function EchProbe() {
  const { control } = useFormContext();
  const ech = useWatch({ control, name: 'streamSettings.tlsSettings.settings.echConfigList' });
  const keys = useWatch({ control, name: 'streamSettings.tlsSettings.echServerKeys' });
  return (
    <>
      <output data-testid="probe-ech">{typeof ech === 'string' ? ech : ''}</output>
      <output data-testid="probe-keys">{typeof keys === 'string' ? keys : ''}</output>
    </>
  );
}

function FormHarness({
  children,
  defaultValues = {},
}: {
  children: ReactNode;
  defaultValues?: Record<string, unknown>;
}) {
  const methods = useForm<InboundFormValues>({ defaultValues: defaultValues as never });
  return (
    <FormProvider {...methods}>
      <Form>
        <EchProbe />
        {children}
      </Form>
    </FormProvider>
  );
}

const noop = () => {};

function renderTls(defaultValues?: Record<string, unknown>) {
  return renderWithProviders(
    <FormHarness defaultValues={defaultValues}>
      <TlsForm
        saving={false}
        setCertFromPanel={noop}
        clearCertFiles={noop}
        pinFromCert={noop}
        pinFromRemote={noop}
        getNewEchCert={noop}
        clearEchCert={noop}
      />
    </FormHarness>,
  );
}

/** The generator is inline now, so it is present as soon as the TLS form renders. */
async function openHelper() {
  return screen.findByTestId('ech-dynamic-helper');
}

describe('inbound TLS dynamic ECH helper', () => {
  it('applies the default Cloudflare + AliDNS value into echConfigList', async () => {
    renderTls();
    const helper = await openHelper();
    expect(within(helper).getByTestId('ech-dynamic-preview').textContent).toBe(
      'cloudflare-ech.com+https://dns.alidns.com/dns-query',
    );
    fireEvent.click(within(helper).getByTestId('ech-dynamic-apply'));

    await waitFor(() =>
      expect(screen.getByTestId('probe-ech').textContent).toBe(
        'cloudflare-ech.com+https://dns.alidns.com/dns-query',
      ),
    );
  });

  it('asks for confirmation before replacing an existing local ECH keypair', async () => {
    // The CDN-invalid local key is exactly what this feature exists to prevent
    // shipping, so overwriting it silently would hide the mistake.
    renderTls({
      streamSettings: {
        tlsSettings: {
          echServerKeys: 'LOCAL-PRIVATE-KEY',
          settings: { echConfigList: '' },
        },
      },
    });
    const helper = await openHelper();
    fireEvent.click(within(helper).getByTestId('ech-dynamic-apply'));

    expect(await within(helper).findByTestId('ech-dynamic-overwrite-warning')).toBeTruthy();
    expect(screen.getByTestId('probe-ech').textContent).toBe('');
    expect(screen.getByTestId('probe-keys').textContent).toBe('LOCAL-PRIVATE-KEY');
  });

  it('clears the dead local key once the operator confirms', async () => {
    renderTls({
      streamSettings: {
        tlsSettings: {
          echServerKeys: 'LOCAL-PRIVATE-KEY',
          settings: { echConfigList: '' },
        },
      },
    });
    const helper = await openHelper();
    fireEvent.click(within(helper).getByTestId('ech-dynamic-apply'));
    fireEvent.click(await within(helper).findByText('Replace and clear local key'));

    await waitFor(() =>
      expect(screen.getByTestId('probe-ech').textContent).toBe(
        'cloudflare-ech.com+https://dns.alidns.com/dns-query',
      ),
    );
    expect(screen.getByTestId('probe-keys').textContent).toBe('');
  });

  it('recovers an existing dynamic value instead of starting from defaults', async () => {
    renderTls({
      streamSettings: {
        tlsSettings: {
          settings: { echConfigList: 'crypto.cloudflare.com+https://dns.google/dns-query' },
        },
      },
    });
    const helper = await openHelper();
    expect(within(helper).getByTestId('ech-dynamic-preview').textContent).toBe(
      'crypto.cloudflare.com+https://dns.google/dns-query',
    );
  });

  it('warns that local keys do not survive a CDN proxy', () => {
    renderTls();
    expect(screen.getByTestId('ech-cdn-warning').textContent).toMatch(/Cloudflare CDN/i);
  });
});
