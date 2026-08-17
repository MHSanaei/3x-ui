import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { Button, Modal, message } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';

import { useSecurityActions } from '@/pages/inbounds/form/useSecurityActions';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';
import type { RealityScanResult } from '@/generated/types';
import { HttpUtil, Msg } from '@/utils';

import { renderWithProviders } from './test-utils';

const BLOCKED: RealityScanResult = {
  target: 'nginx:443',
  host: 'nginx',
  ip: '',
  port: 443,
  feasible: false,
  privateTarget: true,
  tls13: false,
  tlsVersion: '',
  h2: false,
  alpn: '',
  x25519: false,
  curveID: '',
  certValid: false,
  certSubject: '',
  certIssuer: '',
  notAfter: '',
  serverNames: [],
  latencyMs: 0,
  reason: 'connection failed: blocked private/internal address 172.18.0.4',
};

const PROBED: RealityScanResult = {
  ...BLOCKED,
  tls13: true,
  tlsVersion: '1.3',
  h2: true,
  alpn: 'h2',
  x25519: true,
  curveID: 'X25519',
  certValid: true,
  certSubject: 'front.example.com',
  certIssuer: 'Acme Co',
  notAfter: '2026-08-01T00:00:00Z',
  serverNames: ['front.example.com'],
  latencyMs: 2,
  feasible: true,
  reason: '',
};

function Harness() {
  const methods = useForm<InboundFormValues>({
    defaultValues: { streamSettings: { realitySettings: { target: 'nginx:443', xver: 0 } } } as never,
  });
  const [messageApi, messageContextHolder] = message.useMessage();
  const [modal, modalContextHolder] = Modal.useModal();
  const { scanRealityTarget } = useSecurityActions({
    methods,
    setSaving: () => {},
    messageApi,
    modal,
    nodeId: null,
    setScanResult: () => {},
    setScanning: () => {},
  });
  return (
    <FormProvider {...methods}>
      {messageContextHolder}
      {modalContextHolder}
      <Button onClick={() => scanRealityTarget()}>run scan</Button>
    </FormProvider>
  );
}

function confirmDialog() {
  return document.querySelector('.ant-modal-confirm');
}

function confirmButton(text: string) {
  const btn = Array.from(document.querySelectorAll('.ant-modal-confirm-btns button'))
    .find((b) => (b.textContent ?? '').trim() === text);
  if (!btn) throw new Error(`confirm button '${text}' not found`);
  return btn;
}

describe('REALITY scan of a private target', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('asks for confirmation and retries with the opt-in', async () => {
    const post = vi.spyOn(HttpUtil, 'post')
      .mockResolvedValueOnce(new Msg(true, '', BLOCKED))
      .mockResolvedValueOnce(new Msg(true, '', PROBED));
    renderWithProviders(<Harness />);

    fireEvent.click(screen.getByText('run scan'));
    await waitFor(() => expect(confirmDialog()).toBeTruthy());
    expect(confirmDialog()?.textContent).toContain('nginx:443');
    expect(post.mock.calls[0][1]).toMatchObject({ target: 'nginx:443', allowPrivate: false });

    fireEvent.click(confirmButton('Confirm'));
    await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
    expect(post.mock.calls[1][1]).toMatchObject({ target: 'nginx:443', allowPrivate: true });
  });

  it('does not probe the private target when the confirmation is cancelled', async () => {
    const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', BLOCKED));
    renderWithProviders(<Harness />);

    fireEvent.click(screen.getByText('run scan'));
    await waitFor(() => expect(confirmDialog()).toBeTruthy());
    fireEvent.click(confirmButton('Cancel'));

    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(post).toHaveBeenCalledTimes(1);
  });
});
