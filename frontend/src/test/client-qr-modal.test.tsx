import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createRef, forwardRef, useImperativeHandle, useState } from 'react';

import type { HappLinkResult } from '@/generated/types';
import type { ClientRecord } from '@/hooks/useClients';
import ClientQrModal from '@/pages/clients/ClientQrModal';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

vi.mock('@/pages/inbounds/qr', () => ({
  QrPanel: ({ value }: { value: string }) => <div data-testid="qr-panel-value">{value}</div>,
}));

const STANDARD_LINK = 'https://panel.example/sub/alpha';
const HAPP_LINK = 'happ://crypt5/encrypted-alpha';
const CLIENT: ClientRecord = { id: 42, email: 'alice@example.com', subId: 'alpha' };
const SUB_SETTINGS = {
  enable: true,
  subURI: 'https://panel.example/sub/',
  subJsonURI: '',
  subJsonEnable: false,
};

interface SubjectProps {
  open: boolean;
  client: ClientRecord | null;
  subSettings: typeof SUB_SETTINGS;
}

interface SubjectHandle {
  update: (patch: Partial<SubjectProps>) => void;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

function success(encryptedLink = HAPP_LINK) {
  return new Msg<HappLinkResult>(true, '', { encryptedLink });
}

const Subject = forwardRef<SubjectHandle, { overrides: Partial<SubjectProps> }>(function Subject(
  { overrides },
  ref,
) {
  const [props, setProps] = useState<SubjectProps>({
    open: true,
    client: CLIENT,
    subSettings: SUB_SETTINGS,
    ...overrides,
  });
  useImperativeHandle(
    ref,
    () => ({
      update: (patch) => setProps((current) => ({ ...current, ...patch })),
    }),
    [],
  );
  return (
    <ClientQrModal
      open={props.open}
      client={props.client}
      inboundsById={{}}
      subSettings={props.subSettings}
      onOpenChange={vi.fn()}
    />
  );
});

function renderSubject(overrides: Partial<SubjectProps> = {}) {
  const subjectRef = createRef<SubjectHandle>();
  const view = renderWithProviders(<Subject ref={subjectRef} overrides={overrides} />);
  return {
    ...view,
    update(patch: Partial<SubjectProps>) {
      act(() => subjectRef.current?.update(patch));
    },
  };
}

function selectVariant(name: 'Standard' | 'Happ') {
  fireEvent.click(screen.getByRole('radio', { name }));
}

function actionButton(name: 'Retry' | 'Regenerate') {
  return screen.getByRole('button', { name: new RegExp(name) }) as HTMLButtonElement;
}

describe('ClientQrModal Happ presentation', () => {
  beforeEach(() => {
    vi.mocked(HttpUtil.post).mockReset();
  });

  it('opens on Standard without generating a Happ link', () => {
    renderSubject();

    expect((screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement).checked).toBe(
      true,
    );
    expect(screen.getByTestId('qr-panel-value').textContent).toBe(STANDARD_LINK);
    expect(HttpUtil.post).not.toHaveBeenCalled();
  });

  it('posts once with no body and silent errors when Standard switches to Happ', async () => {
    const request = deferred<Msg<HappLinkResult>>();
    vi.mocked(HttpUtil.post).mockReturnValue(request.promise);
    renderSubject();

    selectVariant('Happ');

    await waitFor(() => {
      expect(HttpUtil.post).toHaveBeenCalledOnce();
      expect(HttpUtil.post).toHaveBeenCalledWith('/panel/api/clients/happLink/42', undefined, {
        silent: true,
      });
    });
    expect((screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement).disabled).toBe(
      false,
    );
  });

  it('removes the Happ value on leave and makes a fresh request on re-entry', async () => {
    vi.mocked(HttpUtil.post)
      .mockResolvedValueOnce(success())
      .mockResolvedValueOnce(success('happ://crypt5/encrypted-second'));
    renderSubject();

    selectVariant('Happ');
    expect(await screen.findByText(HAPP_LINK)).toBeTruthy();

    selectVariant('Standard');
    expect(screen.queryByText(HAPP_LINK)).toBeNull();
    expect(screen.getByTestId('qr-panel-value').textContent).toBe(STANDARD_LINK);

    selectVariant('Happ');
    expect(await screen.findByText('happ://crypt5/encrypted-second')).toBeTruthy();
    expect(HttpUtil.post).toHaveBeenCalledTimes(2);
  });

  it('resets to Standard across close and reopen without reusing a prior Happ value', async () => {
    vi.mocked(HttpUtil.post).mockResolvedValue(success());
    const view = renderSubject();
    selectVariant('Happ');
    expect(await screen.findByText(HAPP_LINK)).toBeTruthy();

    view.update({ open: false });
    view.update({ open: true });

    expect((screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement).checked).toBe(
      true,
    );
    expect(screen.getByTestId('qr-panel-value').textContent).toBe(STANDARD_LINK);
    expect(screen.queryByText(HAPP_LINK)).toBeNull();
    expect(HttpUtil.post).toHaveBeenCalledOnce();
  });

  it.each([
    ['leaving Happ', (_view: ReturnType<typeof renderSubject>) => selectVariant('Standard')],
    ['closing', (view: ReturnType<typeof renderSubject>) => view.update({ open: false })],
    [
      'changing client id',
      (view: ReturnType<typeof renderSubject>) => view.update({ client: { ...CLIENT, id: 77 } }),
    ],
    [
      'changing subId',
      (view: ReturnType<typeof renderSubject>) =>
        view.update({ client: { ...CLIENT, subId: 'beta' } }),
    ],
    [
      'changing the effective subscription source',
      (view: ReturnType<typeof renderSubject>) =>
        view.update({
          subSettings: { ...SUB_SETTINGS, subURI: 'https://other.example/sub/' },
        }),
    ],
  ])('ignores a generation response after %s', async (_name, retire) => {
    const oldRequest = deferred<Msg<HappLinkResult>>();
    const nextRequest = deferred<Msg<HappLinkResult>>();
    vi.mocked(HttpUtil.post)
      .mockReturnValueOnce(oldRequest.promise)
      .mockReturnValue(nextRequest.promise);
    const view = renderSubject();
    selectVariant('Happ');
    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledOnce());

    retire(view);
    await act(async () => {
      oldRequest.resolve(success('happ://crypt5/retired-response'));
      await oldRequest.promise;
    });

    expect(screen.queryByText('happ://crypt5/retired-response')).toBeNull();
  });

  it('shows only the localized generic hint and Retry after a backend failure', async () => {
    vi.mocked(HttpUtil.post).mockResolvedValue(
      new Msg<HappLinkResult>(false, 'provider token leaked by backend', null),
    );
    renderSubject();
    selectVariant('Happ');

    await screen.findByText('Retry');
    expect(actionButton('Retry').disabled).toBe(false);
    expect(
      screen.getByText(
        'The Happ link could not be generated. Retry, or check Overview -> Logs for details.',
      ),
    ).toBeTruthy();
    expect(screen.queryByText(/provider token leaked/i)).toBeNull();
  });

  it('retries with a fresh request and exposes Regenerate after success', async () => {
    vi.mocked(HttpUtil.post)
      .mockResolvedValueOnce(new Msg<HappLinkResult>(false, 'backend detail', null))
      .mockResolvedValueOnce(success());
    renderSubject();
    selectVariant('Happ');

    await screen.findByText('Retry');
    fireEvent.click(actionButton('Retry'));

    expect(await screen.findByText(HAPP_LINK)).toBeTruthy();
    expect(actionButton('Regenerate').disabled).toBe(false);
    expect(HttpUtil.post).toHaveBeenCalledTimes(2);
  });

  it('clears the old QR and disables duplicate regeneration while loading', async () => {
    const regeneration = deferred<Msg<HappLinkResult>>();
    vi.mocked(HttpUtil.post)
      .mockResolvedValueOnce(success())
      .mockReturnValueOnce(regeneration.promise);
    renderSubject();
    selectVariant('Happ');
    expect(await screen.findByText(HAPP_LINK)).toBeTruthy();

    fireEvent.click(actionButton('Regenerate'));

    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledTimes(2));
    expect(screen.queryByTestId('qr-panel-value')).toBeNull();
    expect(actionButton('Regenerate').disabled).toBe(true);
    expect((screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement).disabled).toBe(
      false,
    );
  });

  it('passes a valid encryptedLink unchanged to QrPanel', async () => {
    const exactLink = 'happ://crypt5/AaBbCc-._~';
    vi.mocked(HttpUtil.post).mockResolvedValue(success(exactLink));
    renderSubject();
    selectVariant('Happ');

    expect((await screen.findByTestId('qr-panel-value')).textContent).toBe(exactLink);
  });

  it('treats a malformed success object as a generation failure', async () => {
    vi.mocked(HttpUtil.post).mockResolvedValue(
      new Msg<HappLinkResult>(true, '', {} as HappLinkResult),
    );
    renderSubject();
    selectVariant('Happ');

    await screen.findByText('Retry');
    expect(actionButton('Retry').disabled).toBe(false);
    expect(screen.queryByTestId('qr-panel-value')).toBeNull();
  });
});
