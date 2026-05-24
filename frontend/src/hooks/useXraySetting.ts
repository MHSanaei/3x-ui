import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil, PromiseUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

const DIRTY_POLL_MS = 1000;
const DEFAULT_TEST_URL = 'https://www.google.com/generate_204';

export interface OutboundTrafficRow {
  tag: string;
  up: number;
  down: number;
}

export interface OutboundTestResult {
  success: boolean;
  delay?: number;
  error?: string;
  mode?: string;
  ttfbMs?: number;
  tlsMs?: number;
  connectMs?: number;
  dnsMs?: number;
  statusCode?: number;
  endpoints?: { address: string; delay?: number; success: boolean; error?: string }[];
}

export interface OutboundTestState {
  testing?: boolean;
  result?: OutboundTestResult | null;
  mode?: string;
}

export interface XraySettingsValue {
  inbounds?: unknown[];
  outbounds?: { tag?: string; protocol?: string; settings?: unknown; streamSettings?: unknown }[];
  routing?: {
    rules?: { type?: string; outboundTag?: string; balancerTag?: string; [key: string]: unknown }[];
    balancers?: unknown[];
    domainStrategy?: string;
  };
  dns?: { tag?: string; servers?: unknown[] };
  log?: Record<string, unknown>;
  policy?: { system?: Record<string, boolean> };
  observatory?: unknown;
  burstObservatory?: unknown;
  fakedns?: unknown;
  [key: string]: unknown;
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
  restartResult: string;
  outboundsTraffic: OutboundTrafficRow[];
  outboundTestStates: Record<number, OutboundTestState>;
  testingAll: boolean;
  fetchAll: () => Promise<void>;
  fetchOutboundsTraffic: () => Promise<void>;
  resetOutboundsTraffic: (tag: string) => Promise<void>;
  testOutbound: (
    index: number,
    outbound: unknown,
    mode?: string,
  ) => Promise<OutboundTestResult | null>;
  testAllOutbounds: (mode?: string) => Promise<void>;
  saveAll: () => Promise<void>;
  resetToDefault: () => Promise<void>;
  restartXray: () => Promise<void>;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
  msg?: string;
}

interface XrayConfigPayload {
  xraySetting: XraySettingsValue;
  inboundTags?: string[];
  clientReverseTags?: string[];
  outboundTestUrl?: string;
}

async function fetchXrayConfig(): Promise<XrayConfigPayload> {
  const msg = await HttpUtil.post('/panel/xray/', undefined, { silent: true }) as ApiMsg<string>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load xray config');
  if (typeof msg.obj !== 'string') throw new Error('Malformed xray config response: expected string');
  try {
    return JSON.parse(msg.obj) as XrayConfigPayload;
  } catch (e) {
    const err = e as Error;
    throw new Error(`Malformed xray config response: ${err.message}`, { cause: e });
  }
}

async function fetchOutboundsTraffic(): Promise<OutboundTrafficRow[]> {
  const msg = await HttpUtil.get('/panel/xray/getOutboundsTraffic', undefined, { silent: true }) as ApiMsg<OutboundTrafficRow[]>;
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch outbounds traffic');
  return Array.isArray(msg.obj) ? msg.obj : [];
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
  const [restartResult, setRestartResult] = useState('');
  const [outboundTestStates, setOutboundTestStates] = useState<Record<number, OutboundTestState>>({});
  const [testingAll, setTestingAll] = useState(false);

  const oldXraySettingRef = useRef('');
  const oldOutboundTestUrlRef = useRef('');
  const syncingRef = useRef(false);
  const xraySettingRef = useRef('');
  const outboundTestUrlRef = useRef(outboundTestUrl);
  const templateSettingsRef = useRef<XraySettingsValue | null>(null);

  xraySettingRef.current = xraySetting;
  outboundTestUrlRef.current = outboundTestUrl;
  templateSettingsRef.current = templateSettings;

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
    mutationFn: async () =>
      HttpUtil.post('/panel/xray/update', {
        xraySetting: xraySettingRef.current,
        outboundTestUrl: outboundTestUrlRef.current || DEFAULT_TEST_URL,
      }) as Promise<ApiMsg>,
    onSuccess: (msg) => {
      if (msg?.success) queryClient.invalidateQueries({ queryKey: keys.xray.config() });
    },
  });

  const resetTrafficMut = useMutation({
    mutationFn: (tag: string) =>
      HttpUtil.post('/panel/xray/resetOutboundsTraffic', { tag }) as Promise<ApiMsg>,
    onSuccess: (msg) => {
      if (msg?.success) queryClient.invalidateQueries({ queryKey: keys.xray.outboundsTraffic() });
    },
  });

  const restartMut = useMutation({
    mutationFn: async () => {
      const msg = await HttpUtil.post('/panel/api/server/restartXrayService') as ApiMsg;
      if (!msg?.success) return msg;
      await PromiseUtil.sleep(500);
      const r = await HttpUtil.get('/panel/xray/getXrayResult') as ApiMsg<string>;
      if (r?.success) setRestartResult(r.obj || '');
      return msg;
    },
  });

  const resetDefaultMut = useMutation({
    mutationFn: async () => HttpUtil.get('/panel/setting/getDefaultJsonConfig') as Promise<ApiMsg<XraySettingsValue>>,
    onSuccess: (msg) => {
      if (msg?.success && msg.obj) {
        const cloned = JSON.parse(JSON.stringify(msg.obj));
        setTemplateSettings(cloned);
      }
    },
  });

  const saveAll = useCallback(async () => { await saveMut.mutateAsync(); }, [saveMut]);
  const resetOutboundsTraffic = useCallback(async (tag: string) => { await resetTrafficMut.mutateAsync(tag); }, [resetTrafficMut]);
  const restartXray = useCallback(async () => { await restartMut.mutateAsync(); }, [restartMut]);
  const resetToDefault = useCallback(async () => { await resetDefaultMut.mutateAsync(); }, [resetDefaultMut]);

  const spinning = saveMut.isPending || restartMut.isPending || resetDefaultMut.isPending;

  const testOutbound = useCallback(
    async (index: number, outbound: unknown, mode = 'tcp'): Promise<OutboundTestResult | null> => {
      if (!outbound) return null;
      setOutboundTestStates((prev) => ({
        ...prev,
        [index]: { testing: true, result: null, mode },
      }));
      try {
        const msg = await HttpUtil.post('/panel/xray/testOutbound', {
          outbound: JSON.stringify(outbound),
          allOutbounds: JSON.stringify(templateSettingsRef.current?.outbounds || []),
          mode,
        }) as ApiMsg<OutboundTestResult>;
        if (msg?.success && msg.obj) {
          setOutboundTestStates((prev) => ({
            ...prev,
            [index]: { testing: false, result: msg.obj as OutboundTestResult },
          }));
          return msg.obj;
        }
        setOutboundTestStates((prev) => ({
          ...prev,
          [index]: {
            testing: false,
            result: { success: false, error: msg?.msg || 'Unknown error', mode },
          },
        }));
      } catch (e) {
        setOutboundTestStates((prev) => ({
          ...prev,
          [index]: {
            testing: false,
            result: { success: false, error: String(e), mode },
          },
        }));
      }
      return null;
    },
    [],
  );

  const testAllOutbounds = useCallback(async (mode = 'tcp') => {
    const list = templateSettingsRef.current?.outbounds || [];
    if (list.length === 0 || testingAll) return;
    setTestingAll(true);
    try {
      const concurrency = mode === 'tcp' ? 8 : 1;
      const queue = list
        .map((ob, i) => ({ index: i, outbound: ob }))
        .filter(({ outbound }) => {
          const tag = outbound?.tag;
          const proto = outbound?.protocol;
          if (proto === 'blackhole' || proto === 'loopback' || tag === 'blocked') return false;
          if (mode === 'tcp' && (proto === 'freedom' || proto === 'dns')) return false;
          return true;
        });
      async function worker() {
        while (queue.length > 0) {
          const item = queue.shift();
          if (!item) break;
          await testOutbound(item.index, item.outbound, mode);
        }
      }
      const workers = Array.from(
        { length: Math.min(concurrency, queue.length) },
        () => worker(),
      );
      await Promise.all(workers);
    } finally {
      setTestingAll(false);
    }
  }, [testingAll, testOutbound]);

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
      restartResult,
      outboundsTraffic,
      outboundTestStates,
      testingAll,
      fetchAll,
      fetchOutboundsTraffic: fetchOutboundsTrafficCb,
      resetOutboundsTraffic,
      testOutbound,
      testAllOutbounds,
      saveAll,
      resetToDefault,
      restartXray,
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
      restartResult,
      outboundsTraffic,
      outboundTestStates,
      testingAll,
      fetchAll,
      fetchOutboundsTrafficCb,
      resetOutboundsTraffic,
      testOutbound,
      testAllOutbounds,
      saveAll,
      resetToDefault,
      restartXray,
    ],
  );
}
