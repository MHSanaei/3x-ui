import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';

import type { LogFileList, LogLine, LogPage, LogStats } from '@/generated/types';
import LogsPage from '@/pages/logs/LogsPage';
import { HttpUtil, Msg } from '@/utils';
import { renderWithProviders } from './test-utils';

interface FakePlotHooks {
  hooks: { init: ((u: unknown) => void)[] };
}
const plots = vi.hoisted(
  () => [] as { opts: FakePlotHooks; instance: { cursor: { idx: number } } }[],
);

vi.mock('uplot', () => ({
  default: class {
    static paths = { spline: () => undefined };
    static pxRatio = 1;
    cursor = { idx: -1 };
    over = document.createElement('div');
    constructor(opts: FakePlotHooks) {
      plots.push({ opts, instance: this });
      opts.hooks.init.forEach((hook) => hook(this));
    }
    setData() {}
    setSize() {}
    redraw() {}
    destroy() {}
  },
}));

vi.mock('@/api/queries/useAllSettings', () => ({
  useAllSettings: () => ({ allSetting: {} }),
}));

const FILES: LogFileList = {
  folder: '/var/log/x-ui',
  files: [
    { name: 'access.log', size: 2048, modTime: 1767225600000, compressed: false },
    { name: '3xui.log', size: 1024, modTime: 1767225500000, compressed: false },
  ],
  external: [],
};

const STATS: LogStats = {
  file: 'access.log',
  fileSize: 2048,
  scannedBytes: 2048,
  budgetExhausted: false,
  scannedLines: 42,
  matchedLines: 42,
  errors: 3,
  warnings: 7,
  infos: 30,
  debugs: 0,
  unknown: 2,
  uniqueIps: 11,
  uniqueUsers: 5,
  uniqueHosts: 9,
  uniqueCapped: false,
  firstTime: 1767225600000,
  lastTime: 1767229200000,
  bucketMillis: 60000,
  timeline: [
    { start: 1767225600000, total: 12, errors: 1, warnings: 2 },
    { start: 1767225660000, total: 30, errors: 2, warnings: 5 },
  ],
  topIps: [
    { value: '203.0.113.9', count: 30 },
    { value: '198.51.100.7', count: 12 },
  ],
  topUsers: [{ value: 'alice@example.com', count: 20 }],
  topHosts: [{ value: 'example.com', count: 15 }],
  topProtocols: [{ value: 'tcp', count: 40 }],
  topInbounds: [{ value: 'inbound-443', count: 40 }],
  topStatuses: [{ value: 'accepted', count: 40 }],
  topErrorCategories: [
    { value: 'app/dns', count: 8 },
    { value: 'other', count: 2 },
  ],
  geoAvailable: true,
  unlocatedLines: 4,
  countries: [
    { value: 'DE', count: 1000 },
    { value: 'SG', count: 40 },
    { value: 'FR', count: 2 },
  ],
};

function line(over: Partial<LogLine>): LogLine {
  return {
    offset: 0,
    time: 1767225600000,
    level: 'info',
    ip: '',
    country: '',
    user: '',
    host: '',
    protocol: '',
    inbound: '',
    outbound: '',
    status: '',
    category: '',
    message: '',
    raw: '',
    truncated: false,
    ...over,
  };
}

function page(lines: LogLine[], over: Partial<LogPage> = {}): LogPage {
  return {
    file: 'access.log',
    fileSize: 2048,
    lines,
    nextCursor: 0,
    scannedBytes: 2048,
    budgetExhausted: false,
    ...over,
  };
}

type Params = Record<string, string | number>;
let responses: { files: LogFileList; stats: LogStats; read: (params: Params) => LogPage };
let getSpy: MockInstance<typeof HttpUtil.get>;

function callsTo(path: string): Params[] {
  return getSpy.mock.calls.filter(([url]) => url === path).map(([, params]) => params as Params);
}

function lastCallTo(path: string): Params {
  return callsTo(path).at(-1) as Params;
}

async function renderPage() {
  renderWithProviders(
    <MemoryRouter>
      <LogsPage />
    </MemoryRouter>,
  );
  await act(async () => {});
}

async function openRawTab() {
  fireEvent.click(await screen.findByRole('tab', { name: 'Raw logs' }));
}

beforeEach(() => {
  plots.length = 0;
  responses = {
    files: FILES,
    stats: STATS,
    read: () => page([line({ offset: 10, message: 'hello', raw: 'hello' })]),
  };
  getSpy = vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string, params?: unknown) => {
    switch (url) {
      case '/panel/api/logs/files':
        return new Msg(true, '', responses.files);
      case '/panel/api/logs/stats':
        return new Msg(true, '', responses.stats);
      case '/panel/api/logs/read':
        return new Msg(true, '', responses.read(params as Params));
      default:
        return new Msg(true, '', {});
    }
  });
});

describe('LogsPage', () => {
  it('explains an empty folder and names it, instead of showing an empty chart', async () => {
    responses.files = { ...FILES, files: [] };
    await renderPage();
    expect(await screen.findByText('No log files found')).toBeTruthy();
    expect(screen.getByText('/var/log/x-ui')).toBeTruthy();
    expect(callsTo('/panel/api/logs/stats')).toEqual([]);
    expect(callsTo('/panel/api/logs/read')).toEqual([]);
  });

  it('says where a traffic log is written when it sits outside the folder', async () => {
    responses.files = {
      ...FILES,
      external: [{ kind: 'access', path: './access.log', suggested: '/var/log/x-ui/access.log' }],
    };
    await renderPage();
    expect(await screen.findByText('Some Xray logs are outside the log folder')).toBeTruthy();
    expect(screen.getByText('./access.log')).toBeTruthy();
    expect(screen.getByText('/var/log/x-ui/access.log')).toBeTruthy();
  });

  it('shows a read failure rather than an empty result', async () => {
    getSpy.mockImplementation(async (url: string) => {
      if (url === '/panel/api/logs/stats') return new Msg(false, ' (log file not found)');
      return new Msg(true, '', responses.files);
    });
    await renderPage();
    expect(await screen.findByText('log file not found')).toBeTruthy();
  });

  it('opens the most recently written file with summary cards and charts', async () => {
    await renderPage();
    expect(await screen.findByText('Unique IPs')).toBeTruthy();
    expect(lastCallTo('/panel/api/logs/stats')).toEqual({ file: 'access.log' });
    expect(screen.getByText('11')).toBeTruthy();
    expect(plots).toHaveLength(2);
    expect(screen.getByText('203.0.113.9')).toBeTruthy();
    // Stats and pages scan the file, so the hidden tab must not pay for its query.
    expect(callsTo('/panel/api/logs/read')).toEqual([]);
  });

  it('filters by a top-list row and clears the filter on a second click', async () => {
    await renderPage();
    const row = await screen.findByRole('button', { name: /203\.0\.113\.9/ });
    expect(row.getAttribute('aria-pressed')).toBe('false');

    fireEvent.click(row);
    await waitFor(() =>
      expect(lastCallTo('/panel/api/logs/stats')).toEqual({
        file: 'access.log',
        ip: '203.0.113.9',
      }),
    );
    expect(
      screen.getByRole('button', { name: /203\.0\.113\.9/ }).getAttribute('aria-pressed'),
    ).toBe('true');

    fireEvent.click(screen.getByRole('button', { name: /203\.0\.113\.9/ }));
    await waitFor(() =>
      expect(lastCallTo('/panel/api/logs/stats')).toEqual({ file: 'access.log' }),
    );
  });

  it('does not offer the uncategorized bucket as a filter, since no line carries that text', async () => {
    await renderPage();
    await screen.findByText('Uncategorized');
    expect(screen.queryByRole('button', { name: /Uncategorized/ })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /app\/dns/ }));
    await waitFor(() =>
      expect(lastCallTo('/panel/api/logs/stats')).toEqual({
        file: 'access.log',
        status: 'app/dns',
      }),
    );
  });

  it('narrows the time range to the clicked point of a timeline chart', async () => {
    await renderPage();
    await screen.findByText('Unique IPs');
    await waitFor(() => expect(plots).toHaveLength(2));
    const { instance } = plots[0];
    instance.cursor.idx = 1;
    await act(async () => {
      (instance as unknown as { over: HTMLElement }).over.click();
    });
    await waitFor(() =>
      expect(lastCallTo('/panel/api/logs/stats')).toEqual({
        file: 'access.log',
        since: 1767225660000,
        until: 1767225719999,
      }),
    );
  });

  describe('country map', () => {
    const map = () => screen.getByRole('group', { name: 'Requests by country' });

    it('shades countries by volume and leaves the rest as plain land', async () => {
      const { container } = renderWithProviders(
        <MemoryRouter>
          <LogsPage />
        </MemoryRouter>,
      );
      await screen.findByText('Unique IPs');
      const shape = (code: string) => container.querySelector(`path[data-code="${code}"]`);
      expect(shape('DE')?.getAttribute('class')).toBe('logs-map-bin-5');
      expect(shape('FR')?.getAttribute('class')).toBe('logs-map-bin-1');
      expect(shape('BR')?.getAttribute('class')).toBe('logs-map-land');
    });

    // Natural Earth's small scale has no shape for these, and they are common VPN locations.
    it('marks a country too small to draw, and only when it has traffic', async () => {
      const { container } = renderWithProviders(
        <MemoryRouter>
          <LogsPage />
        </MemoryRouter>,
      );
      await screen.findByText('Unique IPs');
      expect(container.querySelector('circle[data-code="SG"]')).not.toBeNull();
      expect(container.querySelector('circle[data-code="HK"]')).toBeNull();
    });

    it('filters by a clicked country and clears on a second click', async () => {
      await renderPage();
      await screen.findByText('Unique IPs');
      fireEvent.click(within(map()).getByRole('button', { name: 'Germany: 1,000' }));
      await waitFor(() =>
        expect(lastCallTo('/panel/api/logs/stats')).toEqual({ file: 'access.log', country: 'DE' }),
      );
      const germany = within(map()).getByRole('button', { name: 'Germany: 1,000' });
      expect(germany.getAttribute('aria-pressed')).toBe('true');
      expect(germany.getAttribute('class')).toContain('is-active');

      fireEvent.click(germany);
      await waitFor(() =>
        expect(lastCallTo('/panel/api/logs/stats')).toEqual({ file: 'access.log' }),
      );
    });

    it('offers only countries with traffic to the keyboard', async () => {
      await renderPage();
      await screen.findByText('Unique IPs');
      const buttons = within(map()).getAllByRole('button');
      expect(buttons.map((el) => el.getAttribute('aria-label')).sort()).toEqual([
        'France: 2',
        'Germany: 1,000',
        'Singapore: 40',
      ]);
      fireEvent.keyDown(within(map()).getByRole('button', { name: 'France: 2' }), { key: 'Enter' });
      await waitFor(() =>
        expect(lastCallTo('/panel/api/logs/stats')).toEqual({ file: 'access.log', country: 'FR' }),
      );
    });

    it('shows a hover tooltip naming any country, with zero where there is no traffic', async () => {
      const { container } = renderWithProviders(
        <MemoryRouter>
          <LogsPage />
        </MemoryRouter>,
      );
      await screen.findByText('Unique IPs');
      fireEvent.pointerMove(container.querySelector('path[data-code="BR"]') as Element);
      const tooltip = container.querySelector('.logs-map-tooltip');
      expect(tooltip?.textContent).toBe('Brazil0');
      fireEvent.pointerLeave(map());
      expect(container.querySelector('.logs-map-tooltip')).toBeNull();
    });

    it('lists the busiest countries by name and says what could not be placed', async () => {
      await renderPage();
      expect(await screen.findByRole('button', { name: /Singapore\s*40/ })).toBeTruthy();
      expect(screen.getByText(/4 lines have an address with no country/)).toBeTruthy();
    });

    it('explains a missing database instead of drawing an empty map', async () => {
      responses.stats = { ...STATS, geoAvailable: false, countries: [], unlocatedLines: 0 };
      await renderPage();
      expect(await screen.findByText(/geoip\.dat was not found/)).toBeTruthy();
      expect(screen.queryByRole('group', { name: 'Requests by country' })).toBeNull();
    });

    it('stays quiet about geo data for a log that names no clients', async () => {
      responses.stats = { ...STATS, geoAvailable: false, countries: [], uniqueIps: 0 };
      await renderPage();
      await screen.findByText('Unique IPs');
      expect(screen.queryByText(/geoip\.dat was not found/)).toBeNull();
    });

    it('sends a typed country filter and shows the country column in raw logs', async () => {
      responses.read = () =>
        page([line({ offset: 1, ip: '198.51.100.7', country: 'DE', message: 'm', raw: 'm' })]);
      await renderPage();
      await openRawTab();
      await screen.findByRole('columnheader', { name: 'Country' });
      fireEvent.change(screen.getByLabelText('Country'), { target: { value: 'de' } });
      await waitFor(() =>
        expect(lastCallTo('/panel/api/logs/read')).toMatchObject({ country: 'de' }),
      );
    });
  });

  it('warns when only part of a large file was analysed', async () => {
    responses.stats = {
      ...STATS,
      budgetExhausted: true,
      scannedBytes: 67108864,
      fileSize: 134217728,
    };
    await renderPage();
    expect(
      await screen.findByText(
        'Stopped after analysing 64.00 MB: the rest of this file is not included.',
      ),
    ).toBeTruthy();
  });

  it('renders hostile log text as inert text and highlights the search term', async () => {
    const hostile = '<img src=x onerror=alert(1)> connect to example.com failed';
    responses.read = () => page([line({ offset: 10, message: hostile, raw: hostile })]);
    const { container } = renderWithProviders(
      <MemoryRouter>
        <LogsPage />
      </MemoryRouter>,
    );
    await openRawTab();
    await screen.findByText(/connect to/);
    expect(container.querySelector('img[src="x"]')).toBeNull();

    fireEvent.change(screen.getByLabelText('Search all text'), {
      target: { value: 'EXAMPLE.com' },
    });
    await waitFor(() =>
      expect(lastCallTo('/panel/api/logs/read')).toMatchObject({ query: 'EXAMPLE.com' }),
    );
    await waitFor(() => expect(container.querySelector('mark')?.textContent).toBe('example.com'));
    expect(container.querySelector('img[src="x"]')).toBeNull();
  });

  it('debounces typing into one request per pause', async () => {
    await renderPage();
    await openRawTab();
    await waitFor(() => expect(callsTo('/panel/api/logs/read')).toHaveLength(1));
    const input = screen.getByLabelText('IP');
    for (const value of ['1', '10', '10.']) {
      fireEvent.change(input, { target: { value } });
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 50));
      });
    }
    await waitFor(() => expect(lastCallTo('/panel/api/logs/read')).toMatchObject({ ip: '10.' }));
    expect(callsTo('/panel/api/logs/read')).toHaveLength(2);
  });

  it('does not run the overview scan behind the raw tab', async () => {
    await renderPage();
    await openRawTab();
    fireEvent.change(screen.getByLabelText('IP'), { target: { value: '10.0.0.1' } });
    await waitFor(() =>
      expect(lastCallTo('/panel/api/logs/read')).toMatchObject({ ip: '10.0.0.1' }),
    );
    expect(callsTo('/panel/api/logs/stats').some((params) => 'ip' in params)).toBe(false);
  });

  it('sends each search field under its own parameter', async () => {
    await renderPage();
    await openRawTab();
    const fields: [string, string][] = [
      ['User', 'user'],
      ['Host', 'host'],
      ['Inbound', 'inbound'],
      ['Protocol', 'protocol'],
      ['Status or message', 'status'],
    ];
    for (const [label, param] of fields) {
      fireEvent.change(screen.getByLabelText(label), { target: { value: `${param}-term` } });
      await waitFor(() =>
        expect(lastCallTo('/panel/api/logs/read')).toMatchObject({ [param]: `${param}-term` }),
      );
    }
  });

  it('filters to one severity from the level control', async () => {
    await renderPage();
    await screen.findByText('Unique IPs');
    const levels = screen.getByLabelText('Level');
    fireEvent.click(within(levels).getByText('Warning'));
    await waitFor(() =>
      expect(lastCallTo('/panel/api/logs/stats')).toEqual({
        file: 'access.log',
        levels: 'warning',
      }),
    );
  });

  it('pages backwards with the cursor the server returned', async () => {
    responses.read = (params) =>
      params.cursor === 500
        ? page([line({ offset: 400, message: 'older line', raw: 'older line' })])
        : page([line({ offset: 600, message: 'newer line', raw: 'newer line' })], {
            nextCursor: 500,
          });
    await renderPage();
    await openRawTab();
    await screen.findByText('newer line');
    expect(lastCallTo('/panel/api/logs/read')).toMatchObject({ cursor: 0, limit: 200 });

    fireEvent.click(screen.getByRole('button', { name: 'Load older lines' }));
    await screen.findByText('older line');
    expect(lastCallTo('/panel/api/logs/read')).toMatchObject({ cursor: 500 });
    expect(screen.getByText('newer line')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Load older lines' })).toBeNull();
  });

  it('lets a search that ran out of budget continue from where it stopped', async () => {
    responses.read = () =>
      page([], { budgetExhausted: true, nextCursor: 900, scannedBytes: 67108864 });
    await renderPage();
    await openRawTab();
    expect(await screen.findByText(/Searched 64\.00 MB without filling this page/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Load older lines' })).toBeTruthy();
  });

  // A gzip scan runs oldest-first, so past the budget there is nothing older to offer.
  it('says the scan stopped when a page ran out of budget with nowhere to continue', async () => {
    responses.read = () =>
      page([], { budgetExhausted: true, nextCursor: 0, scannedBytes: 67108864 });
    await renderPage();
    await openRawTab();
    expect(
      await screen.findByText(
        'Stopped after analysing 64.00 MB: the rest of this file is not included.',
      ),
    ).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Load older lines' })).toBeNull();
  });

  it('stops offering older lines once the browser holds the cap', async () => {
    const full = Array.from({ length: 5000 }, (_, i) =>
      line({ offset: 5000 - i, message: `m${i}`, raw: `m${i}` }),
    );
    responses.read = () => page(full, { nextCursor: 1 });
    await renderPage();
    await openRawTab();
    expect(await screen.findByText(/Showing the newest 5,000 lines/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Load older lines' })).toBeNull();
  });

  it('expands a row into the raw line with fields that filter on click', async () => {
    const raw = '2026/01/01 00:00:00 from 203.0.113.9:1 accepted tcp:a.example:443';
    responses.read = () =>
      page([line({ offset: 7, ip: '203.0.113.9', message: 'short view', raw, truncated: true })]);
    await renderPage();
    await openRawTab();
    fireEvent.click(await screen.findByText('short view'));
    expect(await screen.findByText(raw)).toBeTruthy();
    expect(screen.getByText('Line shortened')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /IP:\s*203\.0\.113\.9/ }));
    await waitFor(() =>
      expect(lastCallTo('/panel/api/logs/read')).toMatchObject({ ip: '203.0.113.9' }),
    );
  });

  it('hides columns that no loaded line fills, so the message keeps the width', async () => {
    responses.read = () => page([line({ offset: 1, ip: '203.0.113.9', message: 'm', raw: 'm' })]);
    await renderPage();
    await openRawTab();
    await waitFor(() => expect(screen.queryByRole('columnheader', { name: 'Inbound' })).toBeNull());
    expect(screen.getByRole('columnheader', { name: 'IP' })).toBeTruthy();
    expect(screen.getByRole('columnheader', { name: 'Message' })).toBeTruthy();
  });

  it.each([
    ['overview', '/panel/api/logs/stats', async () => {}],
    ['raw', '/panel/api/logs/read', openRawTab],
  ])('polls the %s tab only while auto refresh is on', async (_tab, path, openTab) => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      await renderPage();
      await screen.findByText('Unique IPs');
      await openTab();
      await waitFor(() => expect(callsTo(path).length).toBeGreaterThan(0));
      const before = callsTo(path).length;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(20_000);
      });
      expect(callsTo(path)).toHaveLength(before);

      fireEvent.mouseDown(
        screen.getByLabelText('Auto refresh').closest('.ant-select') as HTMLElement,
      );
      fireEvent.click(await screen.findByText('Every 5s'));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(11_000);
      });
      expect(callsTo(path).length).toBeGreaterThanOrEqual(before + 2);
    } finally {
      vi.useRealTimers();
    }
  });

  it('requests a fresh sliding window for a preset range', async () => {
    const now = 1767300000000;
    const spy = vi.spyOn(Date, 'now').mockReturnValue(now);
    try {
      await renderPage();
      await screen.findByText('Unique IPs');
      fireEvent.mouseDown(
        screen.getByLabelText('Time range').closest('.ant-select') as HTMLElement,
      );
      fireEvent.click(await screen.findByText('Last hour'));
      await waitFor(() =>
        expect(lastCallTo('/panel/api/logs/stats')).toEqual({
          file: 'access.log',
          since: now - 3_600_000,
        }),
      );
    } finally {
      spy.mockRestore();
    }
  });
});
