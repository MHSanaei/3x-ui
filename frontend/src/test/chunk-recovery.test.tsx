import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';

vi.mock('react-dom/client', () => ({
  createRoot: () => ({
    render: () => {},
    unmount: () => {},
  }),
}));

vi.mock('react-router/dom', () => ({
  RouterProvider: () => null,
}));

vi.mock('antd', () => ({
  message: { config: () => {} },
}));

vi.mock('@/api/http-init', () => ({
  setupHttp: () => {},
}));

vi.mock('@/i18n/react', () => ({
  readyI18n: () => Promise.resolve(),
}));

vi.mock('@/hooks/useTheme', () => ({
  ThemeProvider: ({ children }: { children?: unknown }) => children ?? null,
}));

vi.mock('@/api/QueryProvider', () => ({
  QueryProvider: ({ children }: { children?: unknown }) => children ?? null,
}));

vi.mock('@/routes', () => ({
  router: { routes: [] },
}));

const PANEL_HREF = 'http://panel.example/secret/panel/inbounds?tab=1#row';

// Same absolute URL main.tsx sees as import.meta.url. Vite rewrites new URL(..., import.meta.url).
function entryModuleUrl(): string {
  return import.meta.url.replace(/\/src\/test\/chunk-recovery\.test\.tsx$/, '/src/main.tsx');
}

function preloadEvent(): Event {
  return new Event('vite:preloadError', { cancelable: true });
}

function createStorage(options?: {
  onSet?: (key: string, value: string) => void;
  throwOn?: 'getItem' | 'setItem';
}): Storage {
  const map = new Map<string, string>();
  return {
    get length() {
      return map.size;
    },
    clear() {
      map.clear();
    },
    getItem(key: string) {
      if (options?.throwOn === 'getItem') throw new DOMException('denied');
      return map.has(key) ? (map.get(key) ?? null) : null;
    },
    key(index: number) {
      return Array.from(map.keys())[index] ?? null;
    },
    removeItem(key: string) {
      map.delete(key);
    },
    setItem(key: string, value: string) {
      if (options?.throwOn === 'setItem') throw new DOMException('denied');
      options?.onSet?.(key, value);
      map.set(key, String(value));
    },
  };
}

function useStorage(storage: Storage) {
  Object.defineProperty(window, 'sessionStorage', {
    configurable: true,
    value: storage,
  });
}

function storageKeys(): string[] {
  const keys: string[] = [];
  for (let i = 0; i < sessionStorage.length; i += 1) {
    const key = sessionStorage.key(i);
    if (key != null) keys.push(key);
  }
  return keys;
}

function installLocation(reload: ReturnType<typeof vi.fn>, href = PANEL_HREF) {
  const url = new URL(href);
  const assign = vi.fn();
  const replace = vi.fn();
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      href,
      origin: url.origin,
      pathname: url.pathname,
      search: url.search,
      hash: url.hash,
      reload,
      assign,
      replace,
    },
  });
  return { assign, replace };
}

const tracked: Array<{ type: string; listener: EventListenerOrEventListenerObject }> = [];
let restoreAdd: typeof window.addEventListener | undefined;

function startTracking() {
  const current = window.addEventListener.bind(window);
  restoreAdd = current;
  window.addEventListener = ((
    type: string,
    listener: EventListenerOrEventListenerObject | null,
    options?: boolean | AddEventListenerOptions,
  ) => {
    if (listener) tracked.push({ type, listener });
    current(type, listener as EventListenerOrEventListenerObject, options);
  }) as typeof window.addEventListener;
}

function stopTracking() {
  const remove = window.removeEventListener.bind(window);
  for (const entry of tracked) remove(entry.type, entry.listener);
  if (restoreAdd) window.addEventListener = restoreAdd;
  restoreAdd = undefined;
  tracked.length = 0;
}

async function boot(): Promise<void> {
  await import('@/main');
  await Promise.resolve();
}

describe('stale chunk recovery', () => {
  let reload: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.resetModules();
    reload = vi.fn();
    useStorage(createStorage());
    installLocation(reload);
    window.X_UI_BASE_PATH = '/secret/';
    document.body.innerHTML = '<div id="message"></div><div id="app"></div>';
    startTracking();
  });

  afterEach(() => {
    stopTracking();
    delete window.X_UI_BASE_PATH;
    vi.clearAllMocks();
  });

  test('reloads once for a preload failure and ignores a re-entrant one', async () => {
    let nested: Event | undefined;
    useStorage(
      createStorage({
        onSet() {
          if (nested) return;
          nested = preloadEvent();
          window.dispatchEvent(nested);
        },
      }),
    );
    const navigation = installLocation(reload);
    let duringReload = '';
    reload.mockImplementation(() => {
      duringReload = storageKeys().join('\n');
    });

    await boot();
    const event = preloadEvent();
    window.dispatchEvent(event);

    expect(entryModuleUrl()).toMatch(/^(https?:|file:)/);
    expect(duringReload).toContain(entryModuleUrl());
    expect(duringReload).toContain('/secret/');
    expect(reload).toHaveBeenCalledTimes(1);
    expect(event.defaultPrevented).toBe(true);
    expect(nested?.defaultPrevented).toBe(false);
    expect(window.location.href).toBe(PANEL_HREF);
    expect(navigation.assign).not.toHaveBeenCalled();
    expect(navigation.replace).not.toHaveBeenCalled();

    const repeat = preloadEvent();
    window.dispatchEvent(repeat);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(repeat.defaultPrevented).toBe(false);
  });

  test('a second bootstrap of the same entry does not reload or hide the error', async () => {
    await boot();
    window.dispatchEvent(preloadEvent());
    expect(reload).toHaveBeenCalledTimes(1);
    const saved = storageKeys();
    expect(saved).toHaveLength(1);
    expect(saved[0]).toContain(entryModuleUrl());

    vi.resetModules();
    await boot();
    expect(storageKeys()).toEqual(saved);

    const again = preloadEvent();
    window.dispatchEvent(again);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(again.defaultPrevented).toBe(false);
    expect(storageKeys()).toEqual(saved);
  });

  test('another base path and a later entry bundle each recover once', async () => {
    window.X_UI_BASE_PATH = '/alpha/';
    await boot();
    window.dispatchEvent(preloadEvent());
    expect(reload).toHaveBeenCalledTimes(1);
    const [alphaKey] = storageKeys();
    expect(alphaKey).toContain('/alpha/');
    expect(alphaKey).toContain(entryModuleUrl());

    vi.resetModules();
    window.X_UI_BASE_PATH = '/beta/';
    await boot();
    window.dispatchEvent(preloadEvent());
    expect(reload).toHaveBeenCalledTimes(2);
    const both = storageKeys();
    expect(both).toContain(alphaKey);
    const betaKey = both.find((key) => key !== alphaKey);
    expect(betaKey).toContain('/beta/');
    expect(betaKey).toContain(entryModuleUrl());

    sessionStorage.clear();
    const laterEntry = 'https://panel.example/beta/assets/index-later.js';
    const laterKey = betaKey!.replace(entryModuleUrl(), laterEntry);
    expect(laterKey).not.toContain(entryModuleUrl());
    sessionStorage.setItem(laterKey, '1');
    vi.resetModules();
    await boot();
    window.dispatchEvent(preloadEvent());
    expect(reload).toHaveBeenCalledTimes(3);
    const recovered = storageKeys();
    expect(recovered).toContain(laterKey);
    expect(recovered).toContain(betaKey);

    const spent = preloadEvent();
    window.dispatchEvent(spent);
    expect(reload).toHaveBeenCalledTimes(3);
    expect(spent.defaultPrevented).toBe(false);
  });

  test.each(['getItem', 'setItem'] as const)(
    'sessionStorage.%s throwing leaves the preload error unsuppressed',
    async (method) => {
      useStorage(createStorage({ throwOn: method }));
      await expect(boot()).resolves.toBeUndefined();

      const event = preloadEvent();
      expect(() => window.dispatchEvent(event)).not.toThrow();
      expect(reload).not.toHaveBeenCalled();
      expect(event.defaultPrevented).toBe(false);

      const again = preloadEvent();
      window.dispatchEvent(again);
      expect(reload).not.toHaveBeenCalled();
      expect(again.defaultPrevented).toBe(false);
    },
  );

  test('runtime errors and navigation do not reload the document', async () => {
    await boot();
    window.dispatchEvent(new ErrorEvent('error', { message: 'render failed', cancelable: true }));
    window.dispatchEvent(new Event('unhandledrejection', { cancelable: true }));
    window.dispatchEvent(new PopStateEvent('popstate', { state: { page: 'hosts' } }));
    expect(reload).not.toHaveBeenCalled();
    expect(storageKeys()).toEqual([]);
  });
});
