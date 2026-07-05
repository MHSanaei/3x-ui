import { useCallback, useEffect, useRef, useState } from 'react';
import { HttpUtil } from '@/utils';

interface TgBotEnvData {
  values: Record<string, string>;
  order: string[];
}

interface DepCheck {
  name: string;
  available: boolean;
  detail?: string;
}

interface ActionResult {
  success?: boolean;
  msg?: string;
  obj?: any;
}

const POLL_INTERVAL_MS = 3000;
const INSTALL_TIMEOUT_MS = 180000;
const MAX_LIVE_LOG_LINES = 500;

function apiBasePath(): string {
  const raw = (typeof window !== 'undefined' && (window as any).X_UI_BASE_PATH) || '/';
  return raw.replace(/\/+$/, '');
}

export function useTgBot() {
  // --- статус службы ---
  const [running, setRunning] = useState<boolean | null>(null);
  const [statusLoading, setStatusLoading] = useState(true);
  const pollRef = useRef<number | null>(null);

  const refreshStatus = useCallback(async () => {
    try {
      const res: ActionResult = await HttpUtil.get('/panel/api/tgbot/status');
      setRunning(!!res?.obj?.running);
    } catch {
      setRunning(null);
    } finally {
      setStatusLoading(false);
    }
  }, []);

  const [actionLoading, setActionLoading] = useState<'start' | 'stop' | 'restart' | null>(null);

  const runAction = useCallback(async (action: 'start' | 'stop' | 'restart') => {
    setActionLoading(action);
    try {
      const res: ActionResult = await HttpUtil.post(`/panel/api/tgbot/${action}`);
      if (res?.success && typeof res.obj?.running === 'boolean') {
        setRunning(res.obj.running);
      } else {
        await refreshStatus();
      }
      return res;
    } finally {
      setActionLoading(null);
    }
  }, [refreshStatus]);

  // --- .env структурный режим ---
  const [envData, setEnvData] = useState<TgBotEnvData>({ values: {}, order: [] });
  const [envLoading, setEnvLoading] = useState(true);

  const refreshEnv = useCallback(async () => {
    setEnvLoading(true);
    try {
      const res: ActionResult = await HttpUtil.get('/panel/api/tgbot/env');
      if (res?.success) {
        setEnvData({ values: res.obj?.values ?? {}, order: res.obj?.order ?? [] });
      }
    } finally {
      setEnvLoading(false);
    }
  }, []);

  const saveEnvValues = useCallback(async (values: Record<string, string>) => {
    const res: ActionResult = await HttpUtil.post('/panel/api/tgbot/env', { values });
    if (res?.success) await refreshEnv();
    return res;
  }, [refreshEnv]);

  // --- .env сырой режим ---
  const getEnvRaw = useCallback(async () => {
    const res: ActionResult = await HttpUtil.get('/panel/api/tgbot/env/raw');
    return res?.success ? ((res.obj?.content as string) ?? '') : '';
  }, []);

  const saveEnvRaw = useCallback(async (content: string) => {
    const res: ActionResult = await HttpUtil.post('/panel/api/tgbot/env/raw', { content });
    if (res?.success) await refreshEnv();
    return res;
  }, [refreshEnv]);

  // --- зависимости ---
  const [dependencies, setDependencies] = useState<DepCheck[]>([]);
  const [depsLoading, setDepsLoading] = useState(true);

  const refreshDependencies = useCallback(async () => {
    setDepsLoading(true);
    try {
      const res: ActionResult = await HttpUtil.get('/panel/api/tgbot/dependencies');
      if (res?.success) setDependencies(res.obj?.dependencies ?? []);
    } finally {
      setDepsLoading(false);
    }
  }, []);

  // --- статус установки ---
  const [installed, setInstalled] = useState<boolean | null>(null);
  const [installing, setInstalling] = useState(false);
  const [installLog, setInstallLog] = useState('');

  const refreshInstalled = useCallback(async () => {
    const res: ActionResult = await HttpUtil.get('/panel/api/tgbot/installed');
    if (res?.success) setInstalled(!!res.obj?.installed);
  }, []);

  const installBot = useCallback(async () => {
    setInstalling(true);
    setInstallLog('');
    try {
      const res: ActionResult = await HttpUtil.post(
        '/panel/api/tgbot/install',
        {},
        { timeout: INSTALL_TIMEOUT_MS },
      );
      setInstallLog(res?.obj?.log || res?.msg || '');
      if (res?.success) {
        await refreshInstalled();
        await refreshStatus();
      }
      return res;
    } finally {
      setInstalling(false);
    }
  }, [refreshInstalled, refreshStatus]);

  // --- логи: разовая выгрузка (fallback / первичная подгрузка) ---
  const [logs, setLogs] = useState('');
  const [logsLoading, setLogsLoading] = useState(false);

  const refreshLogs = useCallback(async () => {
    setLogsLoading(true);
    try {
      const res: ActionResult = await HttpUtil.get('/panel/api/tgbot/logs?lines=200');
      if (res?.success) setLogs(res.obj?.logs ?? '');
      else setLogs(res?.msg || '');
    } finally {
      setLogsLoading(false);
    }
  }, []);

  // --- логи: живой стрим через SSE ---
  const [liveLines, setLiveLines] = useState<string[]>([]);
  const [streaming, setStreaming] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);

  const stopLogStream = useCallback(() => {
    eventSourceRef.current?.close();
    eventSourceRef.current = null;
    setStreaming(false);
  }, []);

  const startLogStream = useCallback(() => {
    if (eventSourceRef.current) return; // уже запущен
    setLiveLines([]);
    const url = `${apiBasePath()}/panel/api/tgbot/logs/stream`;
    const es = new EventSource(url, { withCredentials: true });

    es.onmessage = (event) => {
      setLiveLines((prev) => {
        const next = [...prev, event.data];
        return next.length > MAX_LIVE_LOG_LINES ? next.slice(-MAX_LIVE_LOG_LINES) : next;
      });
    };

    es.onerror = () => {
      // Сервер закрыл соединение или сеть моргнула — не пытаемся
      // бесконечно реконнектиться сами, просто гасим стрим.
      stopLogStream();
    };

    eventSourceRef.current = es;
    setStreaming(true);
  }, [stopLogStream]);

  // Не оставляем открытое соединение, если страница/компонент размонтируется.
  useEffect(() => () => stopLogStream(), [stopLogStream]);

  // --- начальная загрузка + поллинг статуса в реальном времени ---
  useEffect(() => {
    refreshStatus();
    refreshEnv();
    refreshDependencies();
    refreshInstalled();

    pollRef.current = window.setInterval(refreshStatus, POLL_INTERVAL_MS);
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    // статус
    running,
    statusLoading,
    refreshStatus,
    // управление службой
    actionLoading,
    start: () => runAction('start'),
    stop: () => runAction('stop'),
    restart: () => runAction('restart'),
    // .env
    envData,
    envLoading,
    refreshEnv,
    saveEnvValues,
    getEnvRaw,
    saveEnvRaw,
    // зависимости и установка
    dependencies,
    depsLoading,
    refreshDependencies,
    installed,
    installing,
    installLog,
    installBot,
    refreshInstalled,
    // логи: разовая выгрузка
    logs,
    logsLoading,
    refreshLogs,
    // логи: живой стрим
    liveLines,
    streaming,
    startLogStream,
    stopLogStream,
  };
}
