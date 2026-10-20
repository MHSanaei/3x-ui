import { useState } from 'react';
import { act, fireEvent, render, renderHook, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { DBInbound } from '@/models/dbinbound';
import FallbacksCard from '@/pages/inbounds/form/FallbacksCard';
import { useInboundFallbacks } from '@/pages/inbounds/form/useInboundFallbacks';
import type { FallbackRow } from '@/schemas/forms/inbound-form';
import { HttpUtil, Msg } from '@/utils';

import enUS from '../../../internal/web/translation/en-US.json';

const WARNING = enUS.pages.inbounds.fallbacks.proxyProtocolXver;

type Stream = Record<string, unknown>;

function inbound(id: number, streamSettings: DBInbound['streamSettings'], remark = `ib-${id}`) {
  return new DBInbound({
    id,
    remark,
    protocol: 'vless',
    port: 10000 + id,
    listen: '127.0.0.1',
    streamSettings,
  });
}

function expectDerived(
  streamSettings: DBInbound['streamSettings'],
  expected: { xver: number; name?: string; alpn?: string; path?: string },
) {
  const master = inbound(1, { network: 'tcp' });
  const child = inbound(2, streamSettings);
  const wanted = {
    childId: 2,
    name: expected.name ?? '',
    alpn: expected.alpn ?? '',
    path: expected.path ?? '',
    dest: '',
    xver: expected.xver,
  };

  const added = renderHook(() => useInboundFallbacks(master, [master, child]));
  act(() => added.result.current.addAllFallbacks());
  expect(added.result.current.fallbacks).toEqual([expect.objectContaining(wanted)]);
  expect(added.result.current.fallbackChildOptions).toEqual([
    {
      label: 'ib-2 · vless:10002',
      value: 2,
      acceptsProxyProtocol: expected.xver === 1,
    },
  ]);

  const picked = renderHook(() => useInboundFallbacks(master, [master, child]));
  act(() => picked.result.current.addFallback());
  act(() =>
    picked.result.current.updateFallback(picked.result.current.fallbacks[0].rowKey, {
      childId: 2,
    }),
  );
  expect(picked.result.current.fallbacks[0]).toMatchObject(wanted);
}

const OPT_IN: { label: string; stream: Stream; name: string; alpn: string; path: string }[] = [
  {
    label: 'sockopt',
    stream: {
      network: 'ws',
      sockopt: { acceptProxyProtocol: true },
      wsSettings: { acceptProxyProtocol: false, path: '/vlws' },
      tlsSettings: { serverName: 'sock.example', alpn: ['h2'] },
    },
    name: 'sock.example',
    alpn: 'h2',
    path: '/vlws',
  },
  {
    label: 'tcp',
    stream: {
      network: 'tcp',
      sockopt: { acceptProxyProtocol: false },
      tcpSettings: { acceptProxyProtocol: true },
      tlsSettings: { serverName: 'tcp.example', alpn: ['http/1.1'] },
    },
    name: 'tcp.example',
    alpn: 'http/1.1',
    path: '',
  },
  {
    label: 'raw',
    stream: {
      network: 'raw',
      tcpSettings: { acceptProxyProtocol: true },
      tlsSettings: { serverName: 'raw.example', alpn: ['h2', 'http/1.1'] },
    },
    name: 'raw.example',
    alpn: 'h2,http/1.1',
    path: '',
  },
  {
    label: 'ws',
    stream: {
      network: 'ws',
      sockopt: { acceptProxyProtocol: false },
      wsSettings: { acceptProxyProtocol: true, path: '/vlws' },
      tlsSettings: { serverName: 'ws.example', alpn: ['h2'] },
    },
    name: 'ws.example',
    alpn: 'h2',
    path: '/vlws',
  },
  {
    label: 'httpupgrade',
    stream: {
      network: 'httpupgrade',
      sockopt: { acceptProxyProtocol: false },
      httpupgradeSettings: { acceptProxyProtocol: true, path: '/up' },
      tlsSettings: { serverName: 'up.example', alpn: [] },
    },
    name: 'up.example',
    alpn: '',
    path: '/up',
  },
];

const STAY_OFF: {
  label: string;
  stream: DBInbound['streamSettings'];
  name?: string;
  alpn?: string;
  path?: string;
}[] = [
  {
    label: 'false flags',
    stream: {
      network: 'tcp',
      tcpSettings: { acceptProxyProtocol: false },
      sockopt: { acceptProxyProtocol: false },
      tlsSettings: { serverName: 'tcp.example', alpn: ['h2'] },
    },
    name: 'tcp.example',
    alpn: 'h2',
  },
  {
    label: 'missing flags',
    stream: {
      network: 'httpupgrade',
      httpupgradeSettings: { path: '/up' },
      tlsSettings: { serverName: 'up.example', alpn: ['h2', 'http/1.1'] },
    },
    name: 'up.example',
    alpn: 'h2,http/1.1',
    path: '/up',
  },
  {
    label: 'truthy non-booleans',
    stream: {
      network: 'ws',
      wsSettings: { acceptProxyProtocol: 'true', path: '/p' },
      sockopt: { acceptProxyProtocol: 1 },
      tlsSettings: { serverName: 's.example', alpn: ['http/1.1'] },
    },
    name: 's.example',
    alpn: 'http/1.1',
    path: '/p',
  },
  {
    label: 'an inactive tcp flag on ws',
    stream: {
      network: 'ws',
      tcpSettings: { acceptProxyProtocol: true },
      wsSettings: { acceptProxyProtocol: false, path: '/ws' },
      tlsSettings: { serverName: 'ws.example', alpn: ['h2'] },
    },
    name: 'ws.example',
    alpn: 'h2',
    path: '/ws',
  },
  {
    label: 'an inactive ws flag on grpc',
    stream: {
      network: 'grpc',
      wsSettings: { acceptProxyProtocol: true, path: '/ignored' },
      grpcSettings: { serviceName: 'Svc' },
      tlsSettings: { serverName: 'g.example', alpn: ['h2'] },
    },
    name: 'g.example',
    alpn: 'h2',
    path: 'Svc',
  },
  {
    label: 'null transport blocks',
    stream: { network: 'ws', wsSettings: null, sockopt: null, tlsSettings: null },
  },
  {
    label: 'null transport blocks as json',
    stream: JSON.stringify({ network: 'httpupgrade', httpupgradeSettings: null, sockopt: null }),
  },
  {
    label: 'non-object transport blocks',
    stream: { network: 'tcp', tcpSettings: 'nope', sockopt: ['bad'], tlsSettings: 'nope' },
  },
];

describe('fallback proxy protocol defaults', () => {
  describe.each(['object', 'json'] as const)('streamSettings as %s', (encoding) => {
    it.each(OPT_IN)('$label opts in for a pristine child and Add all', (spec) => {
      const stored = encoding === 'json' ? JSON.stringify(spec.stream) : spec.stream;
      expectDerived(stored, { xver: 1, name: spec.name, alpn: spec.alpn, path: spec.path });
    });
  });

  it.each(STAY_OFF)('$label stays at xver 0', (spec) => {
    expectDerived(spec.stream, {
      xver: 0,
      name: spec.name,
      alpn: spec.alpn,
      path: spec.path,
    });
  });

  it('ignores a missing child and malformed stream settings', () => {
    const master = inbound(1, { network: 'tcp' });
    const bad = inbound(5, '{oops');
    const { result } = renderHook(() => useInboundFallbacks(master, [master, bad]));

    act(() => result.current.addAllFallbacks());
    expect(result.current.fallbacks[0]).toMatchObject({
      childId: 5,
      name: '',
      alpn: '',
      path: '',
      dest: '',
      xver: 0,
    });
    expect(result.current.fallbackChildOptions[0].acceptsProxyProtocol).toBe(false);

    act(() => result.current.addFallback());
    act(() => result.current.updateFallback(result.current.fallbacks[1].rowKey, { childId: 404 }));
    expect(result.current.fallbacks[1]).toMatchObject({
      childId: 404,
      name: '',
      alpn: '',
      path: '',
      xver: 0,
    });
    expect(result.current.fallbacks).toHaveLength(2);
  });

  it('keeps explicit xver edits and does not rewrite a row that already has values', () => {
    const master = inbound(1, { network: 'tcp' });
    const ws = inbound(2, {
      network: 'ws',
      wsSettings: { acceptProxyProtocol: true, path: '/vlws' },
      tlsSettings: { serverName: 'ws.example', alpn: ['h2'] },
    });
    const tcp = inbound(3, {
      network: 'tcp',
      tcpSettings: { acceptProxyProtocol: true },
      tlsSettings: { serverName: 'tcp.example', alpn: ['http/1.1'] },
    });
    const { result } = renderHook(() => useInboundFallbacks(master, [master, ws, tcp]));

    act(() => result.current.addFallback());
    const key = result.current.fallbacks[0].rowKey;
    for (const xver of [1, 2, 0]) {
      act(() => result.current.updateFallback(key, { xver }));
      expect(result.current.fallbacks[0].xver).toBe(xver);
    }

    act(() => result.current.updateFallback(key, { name: 'hand.example', xver: 0 }));
    act(() => result.current.updateFallback(key, { childId: 2 }));
    expect(result.current.fallbacks[0]).toMatchObject({
      childId: 2,
      name: 'hand.example',
      alpn: '',
      path: '',
      xver: 0,
    });

    act(() => result.current.updateFallback(key, { xver: 2 }));
    act(() => result.current.updateFallback(key, { childId: 3 }));
    expect(result.current.fallbacks[0]).toMatchObject({
      childId: 3,
      name: 'hand.example',
      xver: 2,
    });

    act(() => result.current.addAllFallbacks());
    expect(result.current.fallbacks.map((row) => row.childId)).toEqual([3, 2]);
    expect(result.current.fallbacks[0]).toMatchObject({
      childId: 3,
      name: 'hand.example',
      xver: 2,
    });
    expect(result.current.fallbacks[1]).toMatchObject({
      childId: 2,
      name: 'ws.example',
      alpn: 'h2',
      path: '/vlws',
      dest: '',
      xver: 1,
    });
    act(() => result.current.addAllFallbacks());
    expect(result.current.fallbacks).toHaveLength(2);
  });

  it('saves a derived xver and reloads stored rows without migrating them', async () => {
    const master = inbound(1, { network: 'tcp' });
    const child = inbound(
      2,
      JSON.stringify({
        network: 'ws',
        wsSettings: { acceptProxyProtocol: true, path: '/vlws' },
        tlsSettings: { serverName: 'ws.example', alpn: ['h2'] },
      }),
    );
    const posted: unknown[] = [];
    vi.spyOn(HttpUtil, 'post').mockImplementation(async (_url, data) => {
      posted.push(data);
      return new Msg(true, '', null);
    });
    vi.spyOn(HttpUtil, 'get').mockImplementation(async () => {
      const body = posted[0] as { fallbacks: unknown[] };
      return new Msg(true, '', body.fallbacks);
    });

    const { result } = renderHook(() => useInboundFallbacks(master, [master, child]));
    act(() => result.current.addAllFallbacks());
    await act(async () => {
      expect(await result.current.saveFallbacks(1)).toBe(true);
    });
    expect(posted).toEqual([
      {
        fallbacks: [
          {
            childId: 2,
            name: 'ws.example',
            alpn: 'h2',
            path: '/vlws',
            dest: '',
            xver: 1,
            sortOrder: 0,
          },
        ],
      },
    ]);

    await act(async () => {
      await result.current.loadFallbacks(1);
    });
    expect(result.current.fallbacks[0]).toMatchObject({
      childId: 2,
      name: 'ws.example',
      alpn: 'h2',
      path: '/vlws',
      dest: '',
      xver: 1,
    });

    vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      new Msg(true, '', [
        { childId: 2, name: 'kept.example', alpn: 'http/1.1', path: '/old', dest: '', xver: 0 },
        { childId: 2, name: 'explicit', alpn: '', path: '', dest: '8443', xver: 2 },
      ]),
    );
    await act(async () => {
      await result.current.loadFallbacks(1);
    });
    expect(result.current.fallbacks.map(({ rowKey: _rowKey, ...row }) => row)).toEqual([
      {
        childId: 2,
        name: 'kept.example',
        alpn: 'http/1.1',
        path: '/old',
        dest: '',
        xver: 0,
      },
      { childId: 2, name: 'explicit', alpn: '', path: '', dest: '8443', xver: 2 },
    ]);
    expect(result.current.fallbackChildOptions[0].acceptsProxyProtocol).toBe(true);

    posted.length = 0;
    await act(async () => {
      await result.current.saveFallbacks(1);
    });
    expect(posted).toEqual([
      {
        fallbacks: [
          {
            childId: 2,
            name: 'kept.example',
            alpn: 'http/1.1',
            path: '/old',
            dest: '',
            xver: 0,
            sortOrder: 0,
          },
          {
            childId: 2,
            name: 'explicit',
            alpn: '',
            path: '',
            dest: '8443',
            xver: 2,
            sortOrder: 1,
          },
        ],
      },
    ]);
  });
});

function EditableCard({
  options,
  initial,
}: {
  options: ReturnType<typeof useInboundFallbacks>['fallbackChildOptions'];
  initial: FallbackRow[];
}) {
  const [rows, setRows] = useState(initial);
  return (
    <FallbacksCard
      fallbacks={rows}
      fallbackChildOptions={options}
      addFallback={() => {}}
      updateFallback={(rowKey, patch) => {
        setRows((prev) => prev.map((row) => (row.rowKey === rowKey ? { ...row, ...patch } : row)));
      }}
      removeFallback={() => {}}
      moveFallback={() => {}}
      addAllFallbacks={() => {}}
    />
  );
}

describe('FallbacksCard proxy protocol warning', () => {
  it('warns only for a linked xver 0 child that accepts PROXY protocol', () => {
    const master = inbound(1, { network: 'tcp' });
    const off = inbound(2, {
      network: 'ws',
      wsSettings: { acceptProxyProtocol: false, path: '/vlws' },
    });
    const on = inbound(2, {
      network: 'ws',
      wsSettings: { acceptProxyProtocol: true, path: '/vlws' },
    });
    const plain = inbound(3, { network: 'tcp', tcpSettings: { acceptProxyProtocol: false } });
    const { result, rerender: rerenderHook } = renderHook(
      ({ list }: { list: DBInbound[] }) => useInboundFallbacks(master, list),
      { initialProps: { list: [master, off, plain] } },
    );
    const initial: FallbackRow[] = [
      {
        rowKey: 'linked',
        childId: 2,
        name: 'old.example',
        alpn: '',
        path: '/old',
        dest: '',
        xver: 0,
      },
      {
        rowKey: 'override',
        childId: 2,
        name: '',
        alpn: '',
        path: '',
        dest: '127.0.0.1:8443',
        xver: 0,
      },
      { rowKey: 'missing', childId: 99, name: '', alpn: '', path: '', dest: '', xver: 0 },
      { rowKey: 'plain', childId: 3, name: '', alpn: '', path: '', dest: '', xver: 0 },
    ];

    const { rerender } = render(
      <EditableCard options={result.current.fallbackChildOptions} initial={initial} />,
    );
    expect(screen.queryByText(WARNING)).toBeNull();

    rerenderHook({ list: [master, on, plain] });
    rerender(<EditableCard options={result.current.fallbackChildOptions} initial={initial} />);
    expect(screen.getAllByText(WARNING)).toHaveLength(1);
    expect(result.current.fallbackChildOptions).toEqual([
      expect.objectContaining({ value: 2, acceptsProxyProtocol: true }),
      expect.objectContaining({ value: 3, acceptsProxyProtocol: false }),
    ]);

    fireEvent.change(screen.getAllByRole('spinbutton')[0], { target: { value: '1' } });
    expect(screen.queryByText(WARNING)).toBeNull();
    fireEvent.change(screen.getAllByRole('spinbutton')[0], { target: { value: '2' } });
    expect(screen.queryByText(WARNING)).toBeNull();
    fireEvent.change(screen.getAllByRole('spinbutton')[0], { target: { value: '0' } });
    expect(screen.getAllByText(WARNING)).toHaveLength(1);
  });
});
