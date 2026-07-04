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
// Установка (git clone + venv + pip install) может занимать долго —
// если HttpUtil поддерживает опции запроса, используем повышенный timeout.
const INSTALL_TIMEOUT_MS = 180000;

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
      // Если HttpUtil.post не поддерживает третий аргумент с опциями —
      // убери его; тогда таймаут будет глобальным (проверь, что он >= 3 мин).
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
  };
}
