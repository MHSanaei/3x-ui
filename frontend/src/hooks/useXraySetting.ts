import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { HttpUtil, PromiseUtil } from '@/utils';

const DIRTY_POLL_MS = 1000;

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
  applyOutboundsEvent: (payload: unknown) => void;
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

export function useXraySetting(): UseXraySettingResult {
  const [fetched, setFetched] = useState(false);
  const [spinning, setSpinning] = useState(false);
  const [saveDisabled, setSaveDisabled] = useState(true);
  const [fetchError, setFetchError] = useState('');
  const [xraySetting, setXraySettingState] = useState('');
  const [templateSettings, setTemplateSettingsState] = useState<XraySettingsValue | null>(null);
  const [outboundTestUrl, setOutboundTestUrlState] = useState('https://www.google.com/generate_204');
  const [inboundTags, setInboundTags] = useState<string[]>([]);
  const [clientReverseTags, setClientReverseTags] = useState<string[]>([]);
  const [restartResult, setRestartResult] = useState('');
  const [outboundsTraffic, setOutboundsTraffic] = useState<OutboundTrafficRow[]>([]);
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
    setFetchError('');
    const msg = await HttpUtil.post('/panel/xray/');
    if (!msg?.success) {
      setFetchError(msg?.msg || 'Failed to load xray config');
      setFetched(true);
      return;
    }
    let obj;
    try {
      obj = JSON.parse(msg.obj);
    } catch (e) {
      const err = e as Error;
      setFetchError(`Malformed xray config response: ${err?.message || String(err)}`);
      setFetched(true);
      return;
    }
    const pretty = JSON.stringify(obj.xraySetting, null, 2);
    syncingRef.current = true;
    setXraySettingState(pretty);
    setTemplateSettingsState(obj.xraySetting);
    oldXraySettingRef.current = pretty;
    syncingRef.current = false;
    setInboundTags(obj.inboundTags || []);
    setClientReverseTags(obj.clientReverseTags || []);
    const nextUrl = obj.outboundTestUrl || 'https://www.google.com/generate_204';
    setOutboundTestUrlState(nextUrl);
    oldOutboundTestUrlRef.current = nextUrl;
    setFetched(true);
    setSaveDisabled(true);
  }, []);

  const saveAll = useCallback(async () => {
    setSpinning(true);
    try {
      const msg = await HttpUtil.post('/panel/xray/update', {
        xraySetting: xraySettingRef.current,
        outboundTestUrl: outboundTestUrlRef.current || 'https://www.google.com/generate_204',
      });
      if (msg?.success) await fetchAll();
    } finally {
      setSpinning(false);
    }
  }, [fetchAll]);

  const fetchOutboundsTraffic = useCallback(async () => {
    const msg = await HttpUtil.get('/panel/xray/getOutboundsTraffic');
    if (msg?.success) setOutboundsTraffic(msg.obj || []);
  }, []);

  const resetOutboundsTraffic = useCallback(async (tag: string) => {
    const msg = await HttpUtil.post('/panel/xray/resetOutboundsTraffic', { tag });
    if (msg?.success) await fetchOutboundsTraffic();
  }, [fetchOutboundsTraffic]);

  const applyOutboundsEvent = useCallback((payload: unknown) => {
    if (Array.isArray(payload)) setOutboundsTraffic(payload as OutboundTrafficRow[]);
  }, []);

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
        });
        if (msg?.success) {
          setOutboundTestStates((prev) => ({
            ...prev,
            [index]: { testing: false, result: msg.obj },
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

  const resetToDefault = useCallback(async () => {
    setSpinning(true);
    try {
      const msg = await HttpUtil.get('/panel/setting/getDefaultJsonConfig');
      if (msg?.success) {
        const cloned = JSON.parse(JSON.stringify(msg.obj));
        setTemplateSettings(cloned);
      }
    } finally {
      setSpinning(false);
    }
  }, [setTemplateSettings]);

  const restartXray = useCallback(async () => {
    setSpinning(true);
    try {
      const msg = await HttpUtil.post('/panel/api/server/restartXrayService');
      if (msg?.success) {
        await PromiseUtil.sleep(500);
        const r = await HttpUtil.get('/panel/xray/getXrayResult');
        if (r?.success) setRestartResult(r.obj || '');
      }
    } finally {
      setSpinning(false);
    }
  }, []);

  useEffect(() => {
    fetchAll();
    fetchOutboundsTraffic();
    const timer = window.setInterval(() => {
      const dirtyXray = oldXraySettingRef.current !== xraySettingRef.current;
      const dirtyUrl = oldOutboundTestUrlRef.current !== outboundTestUrlRef.current;
      setSaveDisabled(!(dirtyXray || dirtyUrl));
    }, DIRTY_POLL_MS);
    return () => window.clearInterval(timer);
  }, [fetchAll, fetchOutboundsTraffic]);

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
      fetchOutboundsTraffic,
      resetOutboundsTraffic,
      applyOutboundsEvent,
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
      fetchOutboundsTraffic,
      resetOutboundsTraffic,
      applyOutboundsEvent,
      testOutbound,
      testAllOutbounds,
      saveAll,
      resetToDefault,
      restartXray,
    ],
  );
}
