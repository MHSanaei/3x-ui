import { useEffect, useState, type ReactNode } from 'react';
import type { Decorator, Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, within } from 'storybook/test';
import { Space } from 'antd';

import { parseTokens } from '@/lib/xray/geoTokens';
import type { GeoCategory, GeoEntry, GeoFile, GeodataTokenIssue } from '@/generated/types';

import GeoTokenInput, { type GeoTokenInputProps } from './GeoTokenInput';

type GeoResponder = (query: URLSearchParams, body: URLSearchParams) => unknown;
type GeoRoutes = Record<string, GeoResponder>;

const realFetch = window.fetch.bind(window);
let activeRoutes: GeoRoutes = {};

function requestUrl(input: RequestInfo | URL): URL {
  if (typeof input === 'string') return new URL(input, window.location.origin);
  if (input instanceof URL) return input;
  return new URL(input.url, window.location.origin);
}

function geoFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const url = requestUrl(input);
  const responder = activeRoutes[url.pathname];
  if (!responder) return realFetch(input, init);
  const form = new URLSearchParams(typeof init?.body === 'string' ? init.body : '');
  const body = JSON.stringify({ success: true, msg: '', obj: responder(url.searchParams, form) });
  return Promise.resolve(
    new Response(body, { status: 200, headers: { 'content-type': 'application/json' } }),
  );
}

function activate(routes: GeoRoutes): void {
  activeRoutes = routes;
  window.fetch = geoFetch;
}

function deactivate(routes: GeoRoutes): void {
  if (activeRoutes === routes) activeRoutes = {};
}

function GeoApi({ routes, children }: { routes: GeoRoutes; children: ReactNode }) {
  const [client] = useState(() => {
    activate(routes);
    return new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
  });
  useEffect(() => {
    activate(routes);
    return () => deactivate(routes);
  }, [routes]);
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

const domain = (value: string): GeoEntry => ({ kind: 'domain', value });
const cidr = (value: string): GeoEntry => ({ kind: 'cidr', value });

const SITE_ENTRIES: Record<string, GeoEntry[]> = {
  'category-ads-all': [
    domain('doubleclick.net'),
    domain('googleadservices.com'),
    domain('googlesyndication.com'),
    domain('criteo.com'),
    domain('taboola.com'),
    domain('outbrain.com'),
  ],
  cn: [
    domain('baidu.com'),
    domain('qq.com'),
    domain('taobao.com'),
    domain('weibo.com'),
    domain('bilibili.com'),
  ],
  google: [
    domain('google.com'),
    domain('googleapis.com'),
    domain('gstatic.com'),
    domain('googleusercontent.com'),
    domain('ggpht.com'),
    domain('android.com'),
  ],
  netflix: [
    domain('netflix.com'),
    domain('nflximg.net'),
    domain('nflxvideo.net'),
    domain('fast.com'),
  ],
  telegram: [domain('telegram.org'), domain('t.me'), domain('telesco.pe'), domain('telegra.ph')],
  youtube: [
    domain('youtube.com'),
    domain('youtu.be'),
    domain('ytimg.com'),
    domain('googlevideo.com'),
  ],
};

const IP_ENTRIES: Record<string, GeoEntry[]> = {
  cloudflare: ['104.16.0.0/13', '172.64.0.0/13', '2606:4700::/32'].map(cidr),
  cn: ['1.0.1.0/24', '36.0.0.0/22', '116.0.0.0/9', '2408:8000::/20'].map(cidr),
  private: [
    '10.0.0.0/8',
    '127.0.0.0/8',
    '169.254.0.0/16',
    '172.16.0.0/12',
    '192.168.0.0/16',
    '::1/128',
    'fc00::/7',
    'fe80::/10',
  ].map(cidr),
  telegram: ['91.108.4.0/22', '149.154.160.0/20', '2001:b28:f23d::/48'].map(cidr),
};

const SITE_ATTRIBUTES: Record<string, string[]> = {
  google: ['ads', 'cn'],
  youtube: ['ads'],
};

function categoriesOf(
  entries: Record<string, GeoEntry[]>,
  attributes: Record<string, string[]> = {},
): GeoCategory[] {
  return Object.keys(entries)
    .sort()
    .map((code) => ({ code, entries: entries[code].length, attributes: attributes[code] ?? [] }));
}

const DATASETS: Record<string, { categories: GeoCategory[]; entries: Record<string, GeoEntry[]> }> =
  {
    'geosite.dat': {
      categories: categoriesOf(SITE_ENTRIES, SITE_ATTRIBUTES),
      entries: SITE_ENTRIES,
    },
    'geoip.dat': { categories: categoriesOf(IP_ENTRIES), entries: IP_ENTRIES },
  };

const UPDATED_AT = Date.UTC(2026, 6, 24, 3, 12);

const FILES: GeoFile[] = [
  {
    name: 'geosite.dat',
    kind: 'site',
    size: 4_812_544,
    modifiedAt: UPDATED_AT,
    categories: DATASETS['geosite.dat'].categories.length,
  },
  {
    name: 'geoip.dat',
    kind: 'ip',
    size: 8_694_272,
    modifiedAt: UPDATED_AT,
    categories: DATASETS['geoip.dat'].categories.length,
  },
];

function referenceOf(token: string, isIP: boolean): { file: string; code: string } | null {
  const [prefix, ...rest] = token.split(':');
  const code = (value: string) => value.split('@')[0].toLowerCase();
  if (prefix === 'geosite') return { file: 'geosite.dat', code: code(rest.join(':')) };
  if (prefix === 'geoip') return { file: 'geoip.dat', code: code(rest.join(':')) };
  if (prefix === 'ext') return { file: rest[0] ?? '', code: code(rest.slice(1).join(':')) };
  return isIP && prefix === 'ext-ip'
    ? { file: rest[0] ?? '', code: code(rest.slice(1).join(':')) }
    : null;
}

function validate(tokens: string[], isIP: boolean): GeodataTokenIssue[] {
  const issues: GeodataTokenIssue[] = [];
  for (const token of tokens) {
    const reference = referenceOf(token, isIP);
    if (!reference) continue;
    const dataset = DATASETS[reference.file];
    if (!dataset) {
      issues.push({ token, reason: 'fileMissing', file: reference.file, code: reference.code });
      continue;
    }
    if (!dataset.categories.some((category) => category.code === reference.code)) {
      issues.push({ token, reason: 'categoryMissing', file: reference.file, code: reference.code });
    }
  }
  return issues;
}

const routes: GeoRoutes = {
  '/csrf-token': () => 'storybook-csrf-token',
  '/panel/api/xray/geodata/files': () => FILES,
  '/panel/api/xray/geodata/categories': (query) => {
    const dataset = DATASETS[query.get('file') ?? ''];
    const needle = (query.get('q') ?? '').trim().toLowerCase();
    const items = (dataset?.categories ?? []).filter((category) => category.code.includes(needle));
    return { total: items.length, items };
  },
  '/panel/api/xray/geodata/entries': (query) => {
    const dataset = DATASETS[query.get('file') ?? ''];
    const needle = (query.get('q') ?? '').trim().toLowerCase();
    const matched = (dataset?.entries[query.get('code') ?? ''] ?? []).filter((entry) =>
      entry.value.toLowerCase().includes(needle),
    );
    const offset = Number(query.get('offset') ?? 0);
    const limit = Number(query.get('limit') ?? 100);
    return { total: matched.length, items: matched.slice(offset, offset + limit) };
  },
  '/panel/api/xray/geodata/validate': (_query, form) =>
    validate(parseTokens(form.get('tokens') ?? ''), form.get('kind') === 'ip'),
};

const withGeodata: Decorator = function GeodataBackend(Story) {
  return (
    <GeoApi routes={routes}>
      <Story />
    </GeoApi>
  );
};

function ControlledTokenInput({ value = '', id = 'geo-rule', ...rest }: GeoTokenInputProps) {
  const [current, setCurrent] = useState(value);
  useEffect(() => setCurrent(value), [value]);
  return (
    <Space orientation="vertical" size={4} style={{ width: 460 }}>
      <label htmlFor={id}>{rest.kind === 'ip' ? 'Target IP' : 'Target domain'}</label>
      <GeoTokenInput {...rest} id={id} value={current} onChange={setCurrent} />
    </Space>
  );
}

const meta = {
  title: 'Geodata/GeoTokenInput',
  component: GeoTokenInput,
  tags: ['autodocs'],
  parameters: {
    layout: 'padded',
    a11y: {
      config: {
        rules: [{ id: 'color-contrast', enabled: false }],
      },
    },
    docs: {
      description: {
        component:
          'Routing rule field for the xray rule editor: a comma separated list of domains/CIDRs and `geosite:` / `geoip:` tokens, with a database button in the addon that opens the geo category browser. Typed tokens are validated against the databases on disk after a short pause, and anything the running core would not resolve is called out under the field. The stories answer `/panel/api/xray/geodata/*` from an in-memory fixture, so validation and the browser both work without a panel backend.',
      },
    },
  },
  decorators: [withGeodata],
  args: { kind: 'domain' },
  argTypes: {
    value: { description: 'Comma separated rule string held by the parent form.' },
    onChange: {
      description: 'Called with the full rule string on every edit and on Apply from the browser.',
    },
    onBlur: {
      description: 'Forwarded to the input; used by React Hook Form to mark the field touched.',
    },
    kind: {
      description:
        'Which database the tokens are validated against: `domain` for geosite, `ip` for geoip.',
      control: 'inline-radio',
      options: ['domain', 'ip'],
    },
    placeholder: { description: 'Placeholder shown while the field is empty.' },
    id: { description: 'Input id, linked to the label rendered by the surrounding form field.' },
  },
  render: (args) => <ControlledTokenInput {...args} />,
} satisfies Meta<typeof GeoTokenInput>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: { kind: 'domain', value: '', placeholder: 'geosite:google, example.com' },
};

export const DomainTokens: Story = {
  args: { kind: 'domain', value: 'geosite:google, google.com' },
};

export const IpTokens: Story = {
  args: { kind: 'ip', value: 'geoip:private' },
};

export const UnknownCategory: Story = {
  args: { kind: 'domain', value: 'geosite:blabla, geosite:google' },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      await canvas.findByText(/geosite:blabla/, undefined, { timeout: 3000 }),
    ).toBeVisible();
  },
};
