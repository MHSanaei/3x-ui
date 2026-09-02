import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createRef, forwardRef, useImperativeHandle, useState } from 'react';
import { MemoryRouter, useLocation } from 'react-router';

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
const HAPP_OPTION_LABEL = 'Happ Encrypted Link';
const CLIENT: ClientRecord = { id: 42, email: 'alice@example.com', subId: 'alpha' };
const SUB_SETTINGS = {
  enable: true,
  subURI: 'https://panel.example/sub/',
  subJsonURI: '',
  subJsonEnable: false,
  happLinkEnable: true,
};

type TestSubSettings = Omit<typeof SUB_SETTINGS, 'happLinkEnable'> & {
  happLinkEnable?: boolean;
};

interface SubjectProps {
  open: boolean;
  client: ClientRecord | null;
  subSettings: TestSubSettings;
  onOpenChange: (open: boolean) => void;
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
    onOpenChange: vi.fn(),
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
      onOpenChange={props.onOpenChange}
    />
  );
});

function LocationProbe() {
  const location = useLocation();
  return (
    <output data-testid="location">
      {location.pathname}
      {location.search}
      {location.hash}
    </output>
  );
}

function renderSubject(overrides: Partial<SubjectProps> = {}) {
  const subjectRef = createRef<SubjectHandle>();
  const onOpenChange = vi.fn();
  const view = renderWithProviders(
    <MemoryRouter initialEntries={['/clients']}>
      <Subject ref={subjectRef} overrides={{ onOpenChange, ...overrides }} />
      <LocationProbe />
    </MemoryRouter>,
  );
  return {
    ...view,
    onOpenChange,
    update(patch: Partial<SubjectProps>) {
      act(() => subjectRef.current?.update(patch));
    },
  };
}

function selectVariant(name: 'Standard' | 'Happ') {
  fireEvent.click(screen.getByRole('radio', { name: name === 'Happ' ? /Happ/ : name }));
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

  it('names the Happ option as an encrypted link', () => {
    renderSubject();

    expect(screen.getByRole('radio', { name: HAPP_OPTION_LABEL })).toBeTruthy();
  });

  it.each([
    ['missing', undefined],
    ['false', false],
  ])('marks the selectable Happ option as locked when the gate is %s', (_name, gate) => {
    const subSettings: TestSubSettings = {
      enable: SUB_SETTINGS.enable,
      subURI: SUB_SETTINGS.subURI,
      subJsonURI: SUB_SETTINGS.subJsonURI,
      subJsonEnable: SUB_SETTINGS.subJsonEnable,
    };
    if (gate !== undefined) subSettings.happLinkEnable = gate;

    renderSubject({ subSettings });

    const standard = screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement;
    const happ = screen.getByRole('radio', { name: /Happ Encrypted Link/ }) as HTMLInputElement;
    expect(standard.checked).toBe(true);
    expect(happ.disabled).toBe(false);
    expect(
      screen.getByLabelText('Enable Happ link generation in Settings before using Happ.', {
        selector: '.anticon-lock',
      }),
    ).toBeTruthy();
    expect(HttpUtil.post).not.toHaveBeenCalled();
  });

  it.each([
    ['missing', undefined],
    ['false', false],
  ])(
    'replaces the blank Happ content with a persistent empty state when the gate is %s',
    (_name, gate) => {
      const subSettings: TestSubSettings = {
        enable: SUB_SETTINGS.enable,
        subURI: SUB_SETTINGS.subURI,
        subJsonURI: SUB_SETTINGS.subJsonURI,
        subJsonEnable: SUB_SETTINGS.subJsonEnable,
      };
      if (gate !== undefined) subSettings.happLinkEnable = gate;

      renderSubject({ subSettings });

      const standard = screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement;
      const happ = screen.getByRole('radio', { name: /Happ Encrypted Link/ }) as HTMLInputElement;
      fireEvent.click(happ);

      expect(standard.checked).toBe(false);
      expect(happ.checked).toBe(true);
      expect(screen.getByText('Happ encrypted link generation is not enabled')).toBeTruthy();
      expect(
        screen.getByText(
          "Enabling it sends this client's complete current subscription URL to crypto.happ.su to generate an encrypted happ://crypt5/ subscription link.",
        ),
      ).toBeTruthy();
      expect(screen.getByRole('button', { name: 'Go to Settings' })).toBeTruthy();
      expect(screen.queryByTestId('qr-panel-value')).toBeNull();
      expect(screen.queryByRole('button', { name: /Regenerate|Retry/ })).toBeNull();
      expect(screen.getByRole('dialog').querySelector('[aria-busy="true"]')).toBeNull();
      expect(HttpUtil.post).not.toHaveBeenCalled();
    },
  );

  it('removes the hover and focus tooltip from the locked Happ option', async () => {
    renderSubject({ subSettings: { ...SUB_SETTINGS, happLinkEnable: false } });
    const happ = screen.getByRole('radio', { name: /Happ Encrypted Link/ });
    const happLabel = happ.closest('label');
    expect(happLabel).not.toBeNull();

    fireEvent.mouseEnter(happLabel!);
    fireEvent.focus(happ);
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 250));
    });

    expect(screen.queryByRole('tooltip')).toBeNull();
  });

  it('closes the QR modal and deep-links to Happ settings without generating', () => {
    const view = renderSubject({ subSettings: { ...SUB_SETTINGS, happLinkEnable: false } });
    selectVariant('Happ');

    fireEvent.click(screen.getByRole('button', { name: 'Go to Settings' }));

    expect(view.onOpenChange).toHaveBeenCalledOnce();
    expect(view.onOpenChange).toHaveBeenCalledWith(false);
    expect(screen.getByTestId('location').textContent).toBe(
      '/settings?subscriptionTab=happ#subscription',
    );
    expect(HttpUtil.post).not.toHaveBeenCalled();
  });

  it('returns to Standard without auto-generating when the gate is enabled after selecting Happ', async () => {
    const view = renderSubject({
      subSettings: { ...SUB_SETTINGS, happLinkEnable: false },
    });
    selectVariant('Happ');
    expect((screen.getByRole('radio', { name: /Happ/ }) as HTMLInputElement).checked).toBe(true);
    expect(HttpUtil.post).not.toHaveBeenCalled();

    view.update({ subSettings: { ...SUB_SETTINGS, happLinkEnable: true } });

    await waitFor(() =>
      expect((screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement).checked).toBe(
        true,
      ),
    );
    expect(
      screen.queryByText(
        "Selecting Happ sends this client's complete current subscription URL to crypto.happ.su.",
      ),
    ).toBeNull();
    expect(HttpUtil.post).not.toHaveBeenCalled();
  });

  it('keeps provider disclosure out of Standard and shows it only in Happ', async () => {
    vi.mocked(HttpUtil.post).mockReturnValue(new Promise(() => {}));
    renderSubject();

    expect((screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement).checked).toBe(
      true,
    );
    expect(
      screen.queryByText(
        "Selecting Happ sends this client's complete current subscription URL to crypto.happ.su.",
      ),
    ).toBeNull();

    selectVariant('Happ');

    expect(
      await screen.findByText(
        "Selecting Happ sends this client's complete current subscription URL to crypto.happ.su.",
      ),
    ).toBeTruthy();
    expect(HttpUtil.post).toHaveBeenCalledOnce();
  });

  it('posts once with no body and silent errors when Standard switches to Happ', async () => {
    const request = deferred<Msg<HappLinkResult>>();
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    vi.mocked(HttpUtil.post).mockReturnValue(request.promise);
    renderSubject();
    const dialogCount = screen.queryAllByRole('dialog').length;

    selectVariant('Happ');

    await waitFor(() => {
      expect(HttpUtil.post).toHaveBeenCalledOnce();
      expect(HttpUtil.post).toHaveBeenCalledWith('/panel/api/clients/happLink/42', undefined, {
        silent: true,
      });
    });
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(screen.queryAllByRole('dialog')).toHaveLength(dialogCount);
    confirmSpy.mockRestore();
    expect((screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement).disabled).toBe(
      false,
    );
    expect(screen.queryByRole('button', { name: /Regenerate|Retry/ })).toBeNull();
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

  it('keeps request B current when request A resolves after leaving and re-entering Happ', async () => {
    const requestA = deferred<Msg<HappLinkResult>>();
    const requestB = deferred<Msg<HappLinkResult>>();
    vi.mocked(HttpUtil.post)
      .mockReturnValueOnce(requestA.promise)
      .mockReturnValueOnce(requestB.promise);
    renderSubject();

    selectVariant('Happ');
    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledOnce());
    selectVariant('Standard');
    selectVariant('Happ');
    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledTimes(2));

    await act(async () => {
      requestA.resolve(success('happ://crypt5/request-a'));
      await requestA.promise;
    });
    expect(screen.queryByText('happ://crypt5/request-a')).toBeNull();
    expect(screen.queryByRole('button', { name: /Regenerate|Retry/ })).toBeNull();

    await act(async () => {
      requestB.resolve(success('happ://crypt5/request-b'));
      await requestB.promise;
    });
    expect(await screen.findByText('happ://crypt5/request-b')).toBeTruthy();
    expect(screen.queryByText('happ://crypt5/request-a')).toBeNull();
  });

  it('returns to Standard and ignores an in-flight response when the gate turns off', async () => {
    const request = deferred<Msg<HappLinkResult>>();
    vi.mocked(HttpUtil.post).mockReturnValue(request.promise);
    const view = renderSubject();
    selectVariant('Happ');
    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledOnce());

    view.update({ subSettings: { ...SUB_SETTINGS, happLinkEnable: false } });

    await waitFor(() => {
      expect((screen.getByRole('radio', { name: 'Standard' }) as HTMLInputElement).checked).toBe(
        true,
      );
      expect((screen.getByRole('radio', { name: /Happ/ }) as HTMLInputElement).disabled).toBe(
        false,
      );
    });
    expect(screen.getByTestId('qr-panel-value').textContent).toBe(STANDARD_LINK);
    expect(screen.queryByRole('button', { name: /Regenerate|Retry/ })).toBeNull();
    expect(screen.getByRole('dialog').querySelector('[aria-busy="true"]')).toBeNull();

    await act(async () => {
      request.resolve(success('happ://crypt5/retired-by-gate'));
      await request.promise;
    });
    expect(screen.queryByText('happ://crypt5/retired-by-gate')).toBeNull();
    expect(HttpUtil.post).toHaveBeenCalledOnce();
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

  it.each([
    ['Retry', new Msg<HappLinkResult>(false, 'backend detail', null)],
    ['Regenerate', success()],
  ])('does not let a stale %s action bypass a disabled gate', async (action, response) => {
    vi.mocked(HttpUtil.post).mockResolvedValue(response);
    const view = renderSubject();
    selectVariant('Happ');
    const staleAction = await screen.findByRole('button', { name: new RegExp(action) });

    view.update({ subSettings: { ...SUB_SETTINGS, happLinkEnable: false } });
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: new RegExp(action) })).toBeNull(),
    );
    fireEvent.click(staleAction);

    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledOnce());
  });

  it('clears the old QR and hides duplicate regeneration while loading', async () => {
    const regeneration = deferred<Msg<HappLinkResult>>();
    vi.mocked(HttpUtil.post)
      .mockResolvedValueOnce(success())
      .mockReturnValueOnce(regeneration.promise);
    renderSubject();
    selectVariant('Happ');
    expect(await screen.findByText(HAPP_LINK)).toBeTruthy();
    await waitFor(() => expect(actionButton('Regenerate').disabled).toBe(false));

    fireEvent.click(actionButton('Regenerate'));

    await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledTimes(2));
    expect(screen.queryByTestId('qr-panel-value')).toBeNull();
    expect(screen.queryByRole('button', { name: /Regenerate|Retry/ })).toBeNull();
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

  it.each([
    ['non-string', { encryptedLink: 7 }],
    ['empty payload', { encryptedLink: 'happ://crypt5/' }],
    ['wrong scheme', { encryptedLink: 'https://provider.example/link' }],
    ['whitespace', { encryptedLink: 'happ://crypt5/has space' }],
    ['control character', { encryptedLink: 'happ://crypt5/example\n' }],
  ])('rejects a %s encryptedLink before rendering QrPanel', async (_name, obj) => {
    vi.mocked(HttpUtil.post).mockResolvedValue(new Msg(true, '', obj));
    renderSubject();
    selectVariant('Happ');

    await screen.findByText('Retry');
    expect(actionButton('Retry').disabled).toBe(false);
    expect(screen.queryByTestId('qr-panel-value')).toBeNull();
  });
});
