import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import ClientHwidListModal from '@/components/clients/ClientHwidList';
import { useClientHwids } from '@/hooks/useClientHwids';
import type { ClientHwidInfo } from '@/lib/clients/hwid-log';
import { HttpUtil } from '@/utils';
import { setupHttp } from '@/api/http-init';

const device: ClientHwidInfo = {
  id: 1,
  blocked: false,
  firstSeen: 1,
  lastSeen: 2,
  userAgent: 'Happ',
  deviceOs: 'Android',
  osVersion: '15',
  deviceModel: 'Phone',
  fingerprint: '123456789abc',
};

afterEach(() => vi.restoreAllMocks());

it('offers blocking and unblocking while keeping banned rows protected from removal', async () => {
  const onSetBlocked = vi.fn();
  const props = {
    open: true,
    hwids: [device],
    loading: false,
    clearing: false,
    deletingId: null,
    blockingId: null,
    formatDate: () => 'today',
    onRefresh: vi.fn(),
    onClearAll: vi.fn(),
    onDelete: vi.fn(),
    onClose: vi.fn(),
    onSetBlocked,
  };
  const { rerender } = render(<ClientHwidListModal {...props} />);
  fireEvent.click(screen.getByRole('button', { name: 'Block' }));
  await screen.findByText('Block this device from fetching this subscription?');
  fireEvent.click(screen.getAllByRole('button', { name: 'Block' }).at(-1)!);
  await waitFor(() => expect(onSetBlocked).toHaveBeenCalledWith(1, true));
  rerender(<ClientHwidListModal {...props} hwids={[{ ...device, blocked: true }]} />);
  expect(screen.getByRole('button', { name: 'Remove device' }).hasAttribute('disabled')).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: 'Unblock' }));
  await screen.findByText('Allow this device to fetch this subscription again?');
  fireEvent.click(screen.getAllByRole('button', { name: 'Unblock' }).at(-1)!);
  await waitFor(() => expect(onSetBlocked).toHaveBeenCalledWith(1, false));
});

it('sends explicit true/false and preserves bans when clearing registrations', async () => {
  const post = vi
    .spyOn(HttpUtil, 'post')
    .mockResolvedValue({ success: true, obj: [device] } as never);
  vi.spyOn(HttpUtil, 'delete').mockResolvedValue({ success: true } as never);
  const { result } = renderHook(() => useClientHwids('a@b'));
  await act(() => result.current.loadHwids());
  await act(() => result.current.setHwidBlocked(1, true));
  expect(post).toHaveBeenLastCalledWith(
    '/panel/api/clients/hwids/a%40b/1/block',
    { blocked: true },
    { headers: { 'Content-Type': 'application/json' } },
  );
  expect(result.current.clientHwids[0].blocked).toBe(true);
  await act(() => result.current.clearHwids());
  expect(result.current.clientHwids).toHaveLength(1);
  await act(() => result.current.setHwidBlocked(1, false));
  expect(post).toHaveBeenLastCalledWith(
    '/panel/api/clients/hwids/a%40b/1/block',
    { blocked: false },
    { headers: { 'Content-Type': 'application/json' } },
  );
  expect(result.current.clientHwids[0].blocked).toBe(false);
});

it('sends block and unblock as JSON through the real HTTP client', async () => {
  // Component setup mocks HttpUtil.post; exercise the real serialization here.
  vi.restoreAllMocks();
  document.head.innerHTML = '<meta name="csrf-token" content="hwid-test-token">';
  setupHttp();
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(
    async () =>
      new Response(JSON.stringify({ success: true, msg: '', obj: [device] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
  );
  const { result } = renderHook(() => useClientHwids('a@b'));
  await act(() => result.current.loadHwids());
  await act(() => result.current.setHwidBlocked(1, true));
  await act(() => result.current.setHwidBlocked(1, false));

  const requests = fetchMock.mock.calls.filter(([url]) =>
    String(url).endsWith('/panel/api/clients/hwids/a%40b/1/block'),
  );
  expect(requests).toHaveLength(2);
  requests.forEach(([, init], index) => {
    const headers = new Headers(init?.headers);
    expect(init?.method).toBe('POST');
    expect(headers.get('Content-Type')).toBe('application/json');
    expect(headers.get('X-CSRF-Token')).toBe('hwid-test-token');
    expect(JSON.parse(String(init?.body))).toEqual({ blocked: index === 0 });
  });
  expect(result.current.clientHwids[0].blocked).toBe(false);
});
