import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import AttachClientsModal from '@/pages/inbounds/clients/AttachClientsModal';
import AttachExistingClientsModal from '@/pages/inbounds/clients/AttachExistingClientsModal';
import { DBInbound } from '@/models/dbinbound';
import { HttpUtil } from '@/utils';

import { renderWithProviders } from './test-utils';

function inbound(id: number) {
  return new DBInbound({
    id,
    port: 443,
    listen: '',
    protocol: 'vless',
    remark: 'edge',
    enable: true,
    settings: '{"clients":[{"email":"not-from-api"}]}',
    streamSettings: '',
    sniffing: '',
  });
}

function candidate(id: number, email: string, alreadyAttached = false) {
  return { id, email, alreadyAttached, enable: true, group: 'fixture', comment: '' };
}

const alice = candidate(1, 'alice@test');
const bob = candidate(119, 'bob@test');
const attached = candidate(2, 'attached@test', true);
const page = { items: [alice, attached], eligible: 9999, total: 10000, groups: ['fixture'] };
const url = '/panel/api/clients/attachCandidates';

afterEach(() => vi.restoreAllMocks());

describe('inbound client attachment picker', () => {
  it('loads source membership from the paged API without decoding inbound settings', async () => {
    const get = vi.spyOn(HttpUtil, 'get').mockResolvedValue({ success: true, obj: page } as never);
    renderWithProviders(
      <AttachClientsModal open source={inbound(7)} dbInbounds={[]} onClose={() => {}} />,
    );
    expect(await screen.findByText('alice@test')).toBeTruthy();
    expect(screen.queryByText('not-from-api')).toBeNull();
    expect(get).toHaveBeenCalledWith(
      url,
      expect.objectContaining({ sourceId: 7, page: 1 }),
      expect.anything(),
    );
  });

  it('submits a compact frozen selection with exclusions retained across searches', async () => {
    const get = vi.spyOn(HttpUtil, 'get').mockImplementation(async (path, raw) => {
      if (path.endsWith('/job-123'))
        return {
          success: true,
          obj: { running: false, result: { attached: 9997, skipped: 0, errors: 0 } },
        } as never;
      const p = raw as { freeze?: boolean; search?: string };
      return {
        success: true,
        obj: p.freeze
          ? { ...page, items: [], scopeToken: 'frozen-token', maxId: 10000 }
          : p.search
            ? { ...page, items: [bob], total: 1 }
            : page,
      } as never;
    });
    const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue({
      success: true,
      obj: { jobId: 'job-123' },
    } as never);
    const close = vi.fn();
    renderWithProviders(<AttachExistingClientsModal open target={inbound(8)} onClose={close} />);
    await screen.findByText('alice@test');
    const aliceBox = screen.getByRole('checkbox', { name: 'alice@test' }) as HTMLInputElement;
    expect(aliceBox.checked).toBe(false);
    expect(
      (screen.getByRole('checkbox', { name: 'attached@test' }) as HTMLInputElement).disabled,
    ).toBe(true);
    fireEvent.click(screen.getByRole('button', { name: /select all eligible clients/i }));
    await waitFor(() => expect(aliceBox.checked).toBe(true));
    fireEvent.click(aliceBox);
    fireEvent.change(screen.getByRole('searchbox', { name: /search/i }), {
      target: { value: 'bob' },
    });
    await screen.findByText('bob@test');
    fireEvent.click(screen.getByRole('checkbox', { name: 'bob@test' }));
    expect(screen.getByRole('button', { name: /restore alice@test/i })).toBeTruthy();
    expect(get).toHaveBeenCalledWith(
      url,
      expect.objectContaining({ inboundIds: '8', freeze: true }),
      expect.anything(),
    );

    const submit = screen.getByRole('button', { name: /^attach clients to/i });
    fireEvent.click(submit);
    fireEvent.click(submit);
    await waitFor(() => expect(close).toHaveBeenCalledOnce());
    expect(post).toHaveBeenCalledExactlyOnceWith(
      '/panel/api/clients/bulkAttach',
      {
        inboundIds: [8],
        selection: {
          mode: 'allMatching',
          sourceId: 0,
          scopeToken: 'frozen-token',
          excludedClientIds: [1, 119],
        },
      },
      expect.anything(),
    );
  });
});
