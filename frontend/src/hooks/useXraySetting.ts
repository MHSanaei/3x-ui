import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import {
  OutboundTrafficListSchema,
  OutboundTestResultListSchema,
  XrayConfigPayloadSchema,
  XraySettingsValueSchema,
  type OutboundTestResult,
  type OutboundTrafficRow,
} from '@/schemas/xray';

const DIRTY_POLL_MS = 1000;
const DEFAULT_TEST_URL = 'https://www.google.com/generate_204';
// One HTTP-mode batch request tests this many outbounds through a single
// shared temp xray instance; chunking keeps responses bounded (~15s worst
// case) and lands Test All results progressively.
const HTTP_BATCH_CHUNK = 16;

export function isUdpOutbound(outbound: unknown): boolean {
  const o = outbound as { protocol?: string; streamSettings?: { network?: string } } | null | undefined;
  const p = o?.protocol;
  const n = o?.streamSettings?.network;
  return p === 'wireguard' || p === 'hysteria' || n === 'hysteria' || n === 'kcp' || n === 'quic';
}

export type { OutboundTrafficRow, OutboundTestResult };

export type XraySettingsValue = z.infer<typeof XraySettingsValueSchema>;

export interface OutboundTestState {
  testing?: boolean;
  result?: OutboundTestResult | null;
  mode?: string;
}

export type SetTemplate = (
  next: XraySettingsValue | null | ((prev: XraySettingsValue | null) => XraySettingsValue | null),
) => void;

export interface UseXraySettingResult {
  fetched: boolean;
  spinning: boolean;
  saveDisabled: boolean;
  fetchError: string;
  xraySetting: string;
  setXraySetting: (next: string) => void;
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  outboundTestUrl: string;
  setOutboundTestUrl: (v: string) => void;
  inboundTags: string[];
  clientReverseTags: string[];
  subscriptionOutbounds: unknown[];
  subscriptionOutboundTags: string[];
  outboundsTraffic: OutboundTrafficRow[];
  outboundTestStates: Record<number, OutboundTestState>;
  subscriptionTestStates: Record<string, OutboundTestState>;
  testingAll: boolean;
  fetchAll: () => Promise<void>;
  fetchOutboundsTraffic: () => Promise<void>;
  resetOutboundsTraffic: (tag: string) => Promise<void>;
  testOutbound: (
    index: number,
    outbound: unknown,
    mode?: string,
  ) => Promise<OutboundTestResult | null>;
  testSubscriptionOutbound: (
    tag: string,
    outbound: unknown,
    mode?: string,
  ) => Promise<OutboundTestResult | null>;
  testAllOutbounds: (mode?: string) => Promise<void>;
  saveAll: () => Promise<void>;
  resetToDefault: () => Promise<void>;
}

type XrayConfigPayload = z.infer<typeof XrayConfigPayloadSchema>;

export async function fetchXrayConfig(): Promise<XrayConfigPayload> {
  const msg = await HttpUtil.post('/panel/api/xray/', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load xray config');
  if (typeof msg.obj !== 'string') throw new Error('Malformed xray config response: expected string');
  let parsed: unknown;
  try {
    parsed = JSON.parse(msg.obj);
  } catch (e) {
    const err = e as Error;
    throw new Error(`Malformed xray config response: ${err.message}`, { cause: e });
  }
  const result = XrayConfigPayloadSchema.safeParse(parsed);
  if (!result.success) {
    console.warn('[zod] xray/ config payload failed validation', result.error.issues);
    return parsed as XrayConfigPayload;
  }
  return result.data;
}

async function fetchOutboundsTraffic(): Promise<OutboundTrafficRow[]> {
  const msg = await HttpUtil.get('/panel/api/xray/getOutboundsTraffic', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch outbounds traffic');
  const validated = parseMsg(msg, OutboundTrafficListSchema, 'xray/getOutboundsTraffic');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useXraySetting(): UseXraySettingResult {
  const queryClient = useQueryClient();

  const configQuery = useQuery({
    queryKey: keys.xray.config(),
    queryFn: fetchXrayConfig,
    staleTime: Infinity,
  });

  const trafficQuery = useQuery({
    queryKey: keys.xray.outboundsTraffic(),
    queryFn: fetchOutboundsTraffic,
    staleTime: Infinity,
  });

  const [saveDisabled, setSaveDisabled] = useState(true);
  const [xraySetting, setXraySettingState] = useState('');
  const [templateSettings, setTemplateSettingsState] = useState<XraySettingsValue | null>(null);
  const [outboundTestUrl, setOutboundTestUrlState] = useState(DEFAULT_TEST_URL);
  const [inboundTags, setInboundTags] = useState<string[]>([]);
  const [clientReverseTags, setClientReverseTags] = useState<string[]>([]);
  const [subscriptionOutbounds, setSubscriptionOutbounds] = useState<unknown[]>([]);
  const [subscriptionOutboundTags, setSubscriptionOutboundTags] = useState<string[]>([]);
  const [outboundTestStates, setOutboundTestStates] = useState<Record<number, OutboundTestState>>({});
  // Subscription outbounds aren't in templateSettings.outbounds, so their test
  // results are keyed by tag rather than by index.
  const [subscriptionTestStates, setSubscriptionTestStates] = useState<Record<string, OutboundTestState>>({});
  const [testingAll, setTestingAll] = useState(false);

  const oldXraySettingRef = useRef('');
  const oldOutboundTestUrlRef = useRef('');
  const syncingRef = useRef(false);
  const xraySettingRef = useRef('');
  const outboundTestUrlRef = useRef(outboundTestUrl);
  const templateSettingsRef = useRef<XraySettingsValue | null>(null);
  const subscriptionOutboundsRef = useRef<unknown[]>([]);

  xraySettingRef.current = xraySetting;
  outboundTestUrlRef.current = outboundTestUrl;
  templateSettingsRef.current = templateSettings;
  subscriptionOutboundsRef.current = subscriptionOutbounds;

  // Seed local editor state from the config query. Runs on first fetch and
  // every time the query refetches (e.g. after a successful save).
  useEffect(() => {
    if (!configQuery.data) return;
    const obj = configQuery.data;
    const pretty = JSON.stringify(obj.xraySetting, null, 2);
    syncingRef.current = true;
    setXraySettingState(pretty);
    setTemplateSettingsState(obj.xraySetting);
    oldXraySettingRef.current = pretty;
    syncingRef.current = false;
    setInboundTags(obj.inboundTags || []);
    setClientReverseTags(obj.clientReverseTags || []);
    setSubscriptionOutbounds(obj.subscriptionOutbounds || []);
    setSubscriptionOutboundTags(obj.subscriptionOutboundTags || []);
    const nextUrl = obj.outboundTestUrl || DEFAULT_TEST_URL;
    setOutboundTestUrlState(nextUrl);
    oldOutboundTestUrlRef.current = nextUrl;
    setSaveDisabled(true);
  }, [configQuery.data]);

  const fetched = configQuery.data !== undefined || configQuery.isError;
  const fetchError = configQuery.error ? (configQuery.error as Error).message : '';

  const setXraySetting = useCallback((next: string) => {
    setXraySettingState(next);
    if (syncingRef.current) return;
    try {
      const parsed = JSON.parse(next);
      syncingRef.current = true;
      setTemplateSettingsState(parsed);
      syncingRef.current = false;
    } catch {
      /* ignore — wait for user to finish */
    }
  }, []);

  const setTemplateSettings: SetTemplate = useCallback((nextOrFn) => {
    setTemplateSettingsState((prev) => {
      const next = typeof nextOrFn === 'function' ? nextOrFn(prev) : nextOrFn;
      if (next == null) return next;
      if (!syncingRef.current) {
        try {
          syncingRef.current = true;
          setXraySettingState(JSON.stringify(next, null, 2));
        } finally {
          syncingRef.current = false;
        }
      }
      return next;
    });
  }, []);

  const setOutboundTestUrl = useCallback((v: string) => {
    setOutboundTestUrlState(v);
  }, []);

  const fetchAll = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: keys.xray.config() });
  }, [queryClient]);

  const fetchOutboundsTrafficCb = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: keys.xray.outboundsTraffic() });
  }, [queryClient]);

  const saveMut = useMutation({
    mutationFn: async () => {
      const sentXraySetting = xraySettingRef.current;
      const sentTestUrl = outboundTestUrlRef.current || DEFAULT_TEST_URL;
      const msg = await HttpUtil.post('/panel/api/xray/update', {
        xraySetting: sentXraySetting,
        outboundTestUrl: sentTestUrl,
      });
      return { msg, sentXraySetting, sentTestUrl };
    },
    onSuccess: ({ msg, sentXraySetting, sentTestUrl }) => {
      if (!msg?.success) return;
      oldXraySettingRef.current = sentXraySetting;
      oldOutboundTestUrlRef.current = sentTestUrl;
      setSaveDisabled(true);
      queryClient.invalidateQueries({ queryKey: keys.xray.config() });
    },
  });

  const resetTrafficMut = useMutation({
    mutationFn: (tag: string) =>
      HttpUtil.post('/panel/api/xray/resetOutboundsTraffic', { tag }),
    onSuccess: (msg) => {
      if (msg?.success) queryClient.invalidateQueries({ queryKey: keys.xray.outboundsTraffic() });
    },
  });

  const resetDefaultMut = useMutation({
    mutationFn: async (): Promise<Msg<XraySettingsValue>> => {
      const raw = await HttpUtil.get('/panel/api/setting/getDefaultJsonConfig');
      return parseMsg(raw, XraySettingsValueSchema, 'setting/getDefaultJsonConfig');
    },
    onSuccess: (msg) => {
      if (msg?.success && msg.obj) {
        const cloned = JSON.parse(JSON.stringify(msg.obj));
        setTemplateSettings(cloned);
      }
    },
  });

  const saveAll = useCallback(async () => { await saveMut.mutateAsync(); }, [saveMut]);
  const resetOutboundsTraffic = useCallback(async (tag: string) => { await resetTrafficMut.mutateAsync(tag); }, [resetTrafficMut]);
  const resetToDefault = useCallback(async () => { await resetDefaultMut.mutateAsync(); }, [resetDefaultMut]);

  const spinning = saveMut.isPending || resetDefaultMut.isPending;

  // Shared POST + parse for a batch of outbound tests. The backend probes the
  // whole batch through one shared temp xray instance and returns results in
  // request order; this aligns them by index and shapes failures so every
  // input gets an OutboundTestResult.
  const postOutboundTestBatch = useCallback(
    async (outbounds: unknown[], effMode: string): Promise<OutboundTestResult[]> => {
      const failAll = (error: string): OutboundTestResult[] =>
        outbounds.map(() => ({ success: false, error, mode: effMode }));
      try {
        const raw = await HttpUtil.post('/panel/api/xray/testOutbounds', {
          outbounds: JSON.stringify(outbounds),
          allOutbounds: JSON.stringify(templateSettingsRef.current?.outbounds || []),
          mode: effMode,
        });
        const msg = parseMsg(raw, OutboundTestResultListSchema, 'xray/testOutbounds');
        if (!msg?.success || !Array.isArray(msg.obj)) return failAll(msg?.msg || 'Unknown error');
        const list = msg.obj;
        return outbounds.map((_ob, i) => list[i] ?? { success: false, error: 'Missing result', mode: effMode });
      } catch (e) {
        return failAll(String(e));
      }
    },
    [],
  );

  const testOutbound = useCallback(
    async (index: number, outbound: unknown, mode = 'tcp'): Promise<OutboundTestResult | null> => {
      if (!outbound) return null;
      const effMode = isUdpOutbound(outbound) ? 'http' : mode;
      setOutboundTestStates((prev) => ({
        ...prev,
        [index]: { testing: true, result: null, mode: effMode },
      }));
      const [result] = await postOutboundTestBatch([outbound], effMode);
      setOutboundTestStates((prev) => ({ ...prev, [index]: { testing: false, result } }));
      return result.success ? result : null;
    },
    [postOutboundTestBatch],
  );

  // Test a subscription outbound (not present in templateSettings.outbounds);
  // results are keyed by tag in subscriptionTestStates.
  const testSubscriptionOutbound = useCallback(
    async (tag: string, outbound: unknown, mode = 'tcp'): Promise<OutboundTestResult | null> => {
      if (!outbound || !tag) return null;
      const effMode = isUdpOutbound(outbound) ? 'http' : mode;
      setSubscriptionTestStates((prev) => ({
        ...prev,
        [tag]: { testing: true, result: null, mode: effMode },
      }));
      const [result] = await postOutboundTestBatch([outbound], effMode);
      setSubscriptionTestStates((prev) => ({ ...prev, [tag]: { testing: false, result } }));
      return result.success ? result : null;
    },
    [postOutboundTestBatch],
  );

  const testAllOutbounds = useCallback(async (mode = 'tcp') => {
    // Template outbounds key their results by index (outboundTestStates);
    // subscription outbounds aren't in the template, so they key by tag
    // (subscriptionTestStates). Both go through the same probe endpoint.
    const templateList = templateSettingsRef.current?.outbounds || [];
    const subList = (subscriptionOutboundsRef.current || []) as Array<{ tag?: string; protocol?: string }>;
    if ((templateList.length === 0 && subList.length === 0) || testingAll) return;
    setTestingAll(true);
    try {
      type TcpEntry =
        | { kind: 'tpl'; index: number; outbound: unknown }
        | { kind: 'sub'; tag: string; outbound: unknown };
      const tcpQueue: TcpEntry[] = [];
      // HTTP batches stay homogeneous (all template or all subscription) so a
      // tag shared between a template and a subscription outbound can't collide
      // inside one batch, and each batch's results route to one state map.
      const httpTplQueue: { index: number; outbound: unknown }[] = [];
      const httpSubQueue: { tag: string; outbound: unknown }[] = [];
      const enqueue = (ob: { tag?: string; protocol?: string }, kind: 'tpl' | 'sub', index: number, tag: string) => {
        const proto = ob?.protocol;
        if (proto === 'blackhole' || proto === 'loopback' || ob?.tag === 'blocked') return;
        // freedom ("direct") and dns aren't proxies — skip them in every mode.
        if (proto === 'freedom' || proto === 'dns') return;
        if (kind === 'sub' && !tag) return;
        const toHttp = mode === 'http' || isUdpOutbound(ob);
        if (kind === 'tpl') {
          if (toHttp) httpTplQueue.push({ index, outbound: ob });
          else tcpQueue.push({ kind: 'tpl', index, outbound: ob });
        } else if (toHttp) {
          httpSubQueue.push({ tag, outbound: ob });
        } else {
          tcpQueue.push({ kind: 'sub', tag, outbound: ob });
        }
      };
      templateList.forEach((ob, i) => enqueue(ob, 'tpl', i, ''));
      subList.forEach((ob) => enqueue(ob, 'sub', -1, typeof ob?.tag === 'string' ? ob.tag : ''));

      // TCP probes are dial-only and cheap server-side; per-item requests
      // keep results landing one by one, each routed to its own state map.
      const runTcpLane = async () => {
        const queue = [...tcpQueue];
        const worker = async () => {
          while (queue.length > 0) {
            const item = queue.shift();
            if (!item) break;
            if (item.kind === 'sub') await testSubscriptionOutbound(item.tag, item.outbound, mode);
            else await testOutbound(item.index, item.outbound, mode);
          }
        };
        await Promise.all(Array.from({ length: Math.min(8, queue.length) }, () => worker()));
      };
      // HTTP probes go out as chunked batches — one temp xray spawn per
      // chunk instead of one per outbound, with results landing per chunk.
      const runTplHttpLane = async () => {
        for (let at = 0; at < httpTplQueue.length; at += HTTP_BATCH_CHUNK) {
          const chunk = httpTplQueue.slice(at, at + HTTP_BATCH_CHUNK);
          setOutboundTestStates((prev) => {
            const next = { ...prev };
            for (const item of chunk) next[item.index] = { testing: true, result: null, mode: 'http' };
            return next;
          });
          const results = await postOutboundTestBatch(chunk.map((c) => c.outbound), 'http');
          setOutboundTestStates((prev) => {
            const next = { ...prev };
            chunk.forEach((item, i) => {
              next[item.index] = { testing: false, result: results[i] };
            });
            return next;
          });
        }
      };
      const runSubHttpLane = async () => {
        for (let at = 0; at < httpSubQueue.length; at += HTTP_BATCH_CHUNK) {
          const chunk = httpSubQueue.slice(at, at + HTTP_BATCH_CHUNK);
          setSubscriptionTestStates((prev) => {
            const next = { ...prev };
            for (const item of chunk) next[item.tag] = { testing: true, result: null, mode: 'http' };
            return next;
          });
          const results = await postOutboundTestBatch(chunk.map((c) => c.outbound), 'http');
          setSubscriptionTestStates((prev) => {
            const next = { ...prev };
            chunk.forEach((item, i) => {
              next[item.tag] = { testing: false, result: results[i] };
            });
            return next;
          });
        }
      };
      // HTTP batches must not overlap: the backend serialises them with a
      // non-blocking lock and rejects a second concurrent batch ("Another
      // outbound test is already running"). Run the template and subscription
      // HTTP lanes one after the other; TCP probes don't take that lock, so
      // they still run alongside.
      const runHttpLane = async () => {
        await runTplHttpLane();
        await runSubHttpLane();
      };
      await Promise.all([runTcpLane(), runHttpLane()]);
    } finally {
      setTestingAll(false);
    }
  }, [testingAll, testOutbound, testSubscriptionOutbound, postOutboundTestBatch]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      const dirtyXray = oldXraySettingRef.current !== xraySettingRef.current;
      const dirtyUrl = oldOutboundTestUrlRef.current !== outboundTestUrlRef.current;
      setSaveDisabled(!(dirtyXray || dirtyUrl));
    }, DIRTY_POLL_MS);
    return () => window.clearInterval(timer);
  }, []);

  const outboundsTraffic = useMemo(() => trafficQuery.data ?? [], [trafficQuery.data]);

  return useMemo(
    () => ({
      fetched,
      spinning,
      saveDisabled,
      fetchError,
      xraySetting,
      setXraySetting,
      templateSettings,
      setTemplateSettings,
      outboundTestUrl,
      setOutboundTestUrl,
      inboundTags,
      clientReverseTags,
      subscriptionOutbounds,
      subscriptionOutboundTags,
      outboundsTraffic,
      outboundTestStates,
      subscriptionTestStates,
      testingAll,
      fetchAll,
      fetchOutboundsTraffic: fetchOutboundsTrafficCb,
      resetOutboundsTraffic,
      testOutbound,
      testSubscriptionOutbound,
      testAllOutbounds,
      saveAll,
      resetToDefault,
    }),
    [
      fetched,
      spinning,
      saveDisabled,
      fetchError,
      xraySetting,
      setXraySetting,
      templateSettings,
      setTemplateSettings,
      outboundTestUrl,
      setOutboundTestUrl,
      inboundTags,
      clientReverseTags,
      subscriptionOutbounds,
      subscriptionOutboundTags,
      outboundsTraffic,
      outboundTestStates,
      subscriptionTestStates,
      testingAll,
      fetchAll,
      fetchOutboundsTrafficCb,
      resetOutboundsTraffic,
      testOutbound,
      testSubscriptionOutbound,
      testAllOutbounds,
      saveAll,
      resetToDefault,
    ],
  );
}
