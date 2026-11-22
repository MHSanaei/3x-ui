import { fireEvent, render, screen } from '@testing-library/react';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import { describe, expect, it } from 'vitest';

import { XhttpDownloadForm } from '@/lib/xray/forms/transport/XhttpDownloadForm';

function Harness({ mode, existing }: { mode: string; existing?: object }) {
  const form = useForm({
    defaultValues: { streamSettings: { xhttpSettings: { downloadSettings: existing } } },
  });
  const values = useWatch({ control: form.control });
  return (
    <FormProvider {...form}>
      <XhttpDownloadForm mode={mode} />
      <output data-testid="wire">{JSON.stringify(values)}</output>
    </FormProvider>
  );
}

describe('separate XHTTP download', () => {
  it('creates a download transport and removes it when switched off', () => {
    render(<Harness mode="packet-up" />);
    const toggle = screen.getByRole('switch');
    fireEvent.click(toggle);
    expect(screen.getByTestId('wire').textContent).toContain('"network":"xhttp"');
    fireEvent.click(toggle);
    expect(screen.getByTestId('wire').textContent).not.toContain('downloadSettings');
  });

  it('opens a stored transport and preserves its other TLS options when editing the path', () => {
    render(
      <Harness
        mode="stream-up"
        existing={{
          network: 'xhttp',
          security: 'tls',
          tlsSettings: { alpn: ['h2'] },
          xhttpSettings: { path: '/original' },
        }}
      />,
    );
    fireEvent.change(screen.getByDisplayValue('/original'), { target: { value: '/changed' } });
    const wire = JSON.parse(screen.getByTestId('wire').textContent!);
    expect(wire.streamSettings.xhttpSettings.downloadSettings).toMatchObject({
      tlsSettings: { alpn: ['h2'] },
      xhttpSettings: { path: '/changed' },
    });
  });

  it('disables separate download in stream-one mode', () => {
    render(<Harness mode="stream-one" />);
    expect((screen.getByRole('switch') as HTMLButtonElement).disabled).toBe(true);
  });
});
