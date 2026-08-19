import { useEffect, useState, type ReactNode } from 'react';
import type { Decorator, Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, within } from 'storybook/test';
import { Button, Space, Typography } from 'antd';

import type { GeoCategory, GeoEntry, GeoFile } from '@/generated/types';

import GeoBrowserModal, { type GeoBrowserModalProps } from './GeoBrowserModal';

type GeoResponder = (query: URLSearchParams) => unknown;
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
  const body = JSON.stringify({ success: true, msg: '', obj: responder(url.searchParams) });
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
const full = (value: string): GeoEntry => ({ kind: 'full', value });
const keyword = (value: string): GeoEntry => ({ kind: 'keyword', value });
const regexp = (value: string): GeoEntry => ({ kind: 'regexp', value });
const cidr = (value: string): GeoEntry => ({ kind: 'cidr', value });

const cross = (names: string[], suffixes: string[]): GeoEntry[] =>
  names.flatMap((name) => suffixes.map((suffix) => domain(`${name}.${suffix}`)));

const CC_TLDS = [
  'ae',
  'al',
  'am',
  'at',
  'az',
  'ba',
  'be',
  'bg',
  'bi',
  'bj',
  'ca',
  'cat',
  'cd',
  'cf',
  'cg',
  'ch',
  'ci',
  'cl',
  'cm',
  'co.id',
  'co.il',
  'co.in',
  'co.jp',
  'co.ke',
  'co.kr',
  'co.ma',
  'co.nz',
  'co.th',
  'co.uk',
  'co.uz',
  'co.ve',
  'co.za',
  'com.ar',
  'com.au',
  'com.bd',
  'com.br',
  'com.co',
  'com.cu',
  'com.eg',
  'com.gt',
  'com.hk',
  'com.mx',
  'com.my',
  'com.ng',
  'com.pe',
  'com.ph',
  'com.pk',
  'com.sa',
  'com.sg',
  'com.tr',
  'com.tw',
  'com.ua',
  'com.uy',
  'com.vn',
  'cz',
  'de',
  'dj',
  'dk',
  'dz',
  'ee',
  'es',
  'fi',
  'fr',
  'ga',
  'ge',
  'gl',
  'gm',
  'gr',
  'hn',
  'hr',
  'ht',
  'hu',
  'ie',
  'iq',
  'is',
  'it',
  'je',
  'jo',
  'kg',
  'kz',
  'la',
  'li',
  'lk',
  'lt',
  'lu',
  'lv',
  'ly',
  'md',
  'me',
  'mg',
  'mk',
  'ml',
  'mn',
  'mu',
  'mv',
  'mw',
  'ne',
  'nl',
  'no',
  'nu',
  'pl',
  'pt',
  'ro',
  'rs',
  'ru',
  'rw',
  'se',
  'sh',
  'si',
  'sk',
  'sm',
  'sn',
  'so',
  'sr',
  'st',
  'td',
  'tg',
  'tk',
  'tl',
  'tm',
  'tn',
  'to',
  'tt',
  'vg',
  'vu',
  'ws',
];

const AD_HOSTS = [
  'adform',
  'adnxs',
  'adroll',
  'adsrvr',
  'amplitude',
  'appsflyer',
  'bluekai',
  'branch',
  'casalemedia',
  'criteo',
  'flurry',
  'moatads',
  'mopub',
  'openx',
  'outbrain',
  'pubmatic',
  'quantserve',
  'rubiconproject',
  'scorecardresearch',
  'sharethrough',
  'smartadserver',
  'taboola',
  'teads',
  'yieldmo',
  'zemanta',
];

const CN_BRANDS = [
  '58',
  'alibaba',
  'alipay',
  'aliyun',
  'baidu',
  'bilibili',
  'cnblogs',
  'csdn',
  'ctrip',
  'douban',
  'gitee',
  'huawei',
  'iqiyi',
  'jd',
  'kuaishou',
  'meituan',
  'netease',
  'pinduoduo',
  'qq',
  'sina',
  'sohu',
  'taobao',
  'tencent',
  'tmall',
  'toutiao',
  'weibo',
  'xiaomi',
  'youku',
  'zhihu',
];

const SITE_ENTRIES: Record<string, GeoEntry[]> = {
  amazon: [
    domain('amazon.com'),
    domain('amazonaws.com'),
    domain('media-amazon.com'),
    domain('ssl-images-amazon.com'),
    domain('primevideo.com'),
    domain('awsstatic.com'),
    domain('cloudfront.net'),
    full('www.amazon.co.jp'),
  ],
  apple: [
    domain('apple.com'),
    domain('icloud.com'),
    domain('cdn-apple.com'),
    domain('mzstatic.com'),
    domain('apple-cloudkit.com'),
    domain('itunes.com'),
    domain('me.com'),
    domain('appstore.com'),
  ],
  'category-ads': [
    domain('adcolony.com'),
    domain('applovin.com'),
    domain('chartboost.com'),
    domain('inmobi.com'),
    domain('unityads.unity3d.com'),
    keyword('banner-ad'),
  ],
  'category-ads-all': [
    domain('doubleclick.net'),
    domain('googleadservices.com'),
    domain('googlesyndication.com'),
    domain('adservice.google.com'),
    full('ads.yahoo.com'),
    keyword('adservice'),
    keyword('advertising'),
    regexp('^ad[0-9]{1,3}\\.'),
    ...cross(AD_HOSTS, ['com', 'net', 'io', 'ru']),
  ],
  cloudflare: [
    domain('cloudflare.com'),
    domain('cloudflare-dns.com'),
    domain('cloudflareinsights.com'),
    domain('workers.dev'),
    domain('pages.dev'),
    domain('cf-ipfs.com'),
  ],
  cn: [full('www.gov.cn'), keyword('chinanet'), ...cross(CN_BRANDS, ['com', 'cn', 'com.cn'])],
  discord: [
    domain('discord.com'),
    domain('discord.gg'),
    domain('discordapp.com'),
    domain('discordapp.net'),
    domain('discord.media'),
  ],
  facebook: [
    domain('facebook.com'),
    domain('fbcdn.net'),
    domain('fb.com'),
    domain('messenger.com'),
    domain('fbsbx.com'),
    domain('facebook.net'),
    full('m.facebook.com'),
  ],
  'geolocation-!cn': [
    keyword('proxy'),
    regexp('.*\\.onion$'),
    domain('wikipedia.org'),
    domain('bbc.com'),
    domain('nytimes.com'),
    domain('reuters.com'),
    domain('medium.com'),
    domain('reddit.com'),
  ],
  'geolocation-cn': [
    domain('gov.cn'),
    domain('edu.cn'),
    domain('org.cn'),
    domain('net.cn'),
    ...cross(CN_BRANDS.slice(0, 18), ['cn']),
  ],
  github: [
    domain('github.com'),
    domain('githubusercontent.com'),
    domain('githubassets.com'),
    domain('github.io'),
    domain('ghcr.io'),
    domain('git.io'),
  ],
  google: [
    domain('google.com'),
    domain('googleapis.com'),
    domain('gstatic.com'),
    domain('googleusercontent.com'),
    domain('google-analytics.com'),
    domain('googletagmanager.com'),
    domain('ggpht.com'),
    domain('withgoogle.com'),
    domain('android.com'),
    domain('chromium.org'),
    domain('abc.xyz'),
    full('dl.google.com'),
    ...CC_TLDS.map((tld) => domain(`google.${tld}`)),
  ],
  instagram: [domain('instagram.com'), domain('cdninstagram.com'), domain('ig.me')],
  microsoft: [
    domain('microsoft.com'),
    domain('live.com'),
    domain('office.com'),
    domain('office365.com'),
    domain('windows.net'),
    domain('windowsupdate.com'),
    domain('msn.com'),
    domain('azure.com'),
    domain('sharepoint.com'),
    domain('skype.com'),
    domain('bing.com'),
  ],
  netflix: [
    domain('netflix.com'),
    domain('netflix.net'),
    domain('nflximg.com'),
    domain('nflximg.net'),
    domain('nflxvideo.net'),
    domain('nflxso.net'),
    domain('nflxext.com'),
    full('fast.com'),
  ],
  openai: [
    domain('openai.com'),
    domain('chatgpt.com'),
    domain('oaistatic.com'),
    domain('oaiusercontent.com'),
    domain('sora.com'),
  ],
  spotify: [
    domain('spotify.com'),
    domain('scdn.co'),
    domain('spotifycdn.com'),
    domain('spoti.fi'),
    domain('spotifycdn.net'),
  ],
  steam: [
    domain('steampowered.com'),
    domain('steamcommunity.com'),
    domain('steamstatic.com'),
    domain('steamcontent.com'),
    domain('valvesoftware.com'),
  ],
  telegram: [
    domain('telegram.org'),
    domain('telegram.me'),
    domain('t.me'),
    domain('telesco.pe'),
    domain('tdesktop.com'),
    domain('telegra.ph'),
    domain('cdn-telegram.org'),
    full('comments.app'),
    keyword('telegram'),
  ],
  tiktok: [
    domain('tiktok.com'),
    domain('tiktokcdn.com'),
    domain('tiktokv.com'),
    domain('byteoversea.com'),
    domain('ibytedtos.com'),
    domain('musical.ly'),
  ],
  twitch: [domain('twitch.tv'), domain('ttvnw.net'), domain('jtvnw.net'), domain('twitchcdn.net')],
  twitter: [
    domain('twitter.com'),
    domain('x.com'),
    domain('t.co'),
    domain('twimg.com'),
    domain('periscope.tv'),
  ],
  whatsapp: [domain('whatsapp.com'), domain('whatsapp.net'), domain('wa.me')],
  youtube: [
    domain('youtube.com'),
    domain('youtu.be'),
    domain('ytimg.com'),
    domain('googlevideo.com'),
    domain('youtube-nocookie.com'),
    domain('yt.be'),
  ],
};

const SITE_ATTRIBUTES: Record<string, string[]> = {
  amazon: ['ads'],
  apple: ['cn'],
  facebook: ['ads'],
  google: ['ads', 'cn'],
  instagram: ['ads'],
  microsoft: ['cn'],
  tiktok: ['ads', 'cn'],
  twitter: ['ads'],
  youtube: ['ads'],
};

const CN_BLOCKS = [
  '1.0.1.0/24',
  '1.0.2.0/23',
  '1.0.8.0/21',
  '14.0.12.0/22',
  '27.0.128.0/21',
  '36.0.0.0/22',
  '39.0.0.0/24',
  '42.0.0.0/22',
  '58.14.0.0/15',
  '59.32.0.0/11',
  '61.128.0.0/10',
  '101.16.0.0/12',
  '103.1.8.0/22',
  '106.0.0.0/10',
  '110.6.0.0/15',
  '111.0.0.0/10',
  '112.0.0.0/10',
  '113.0.0.0/9',
  '114.28.0.0/16',
  '116.0.0.0/9',
  '117.8.0.0/13',
  '118.24.0.0/15',
  '119.0.0.0/9',
  '120.0.0.0/10',
  '121.0.0.0/8',
  '124.0.0.0/8',
  '125.32.0.0/11',
  '139.196.0.0/14',
  '140.75.0.0/16',
  '175.0.0.0/12',
  '180.76.0.0/16',
  '182.16.0.0/12',
  '183.0.0.0/10',
  '202.0.0.0/12',
  '203.0.0.0/12',
  '210.0.0.0/12',
  '211.64.0.0/11',
  '218.0.0.0/9',
  '219.72.0.0/14',
  '220.112.0.0/12',
  '221.0.0.0/9',
  '222.16.0.0/12',
  '2001:250::/35',
  '2400:3200::/32',
  '2408:8000::/20',
];

const CN_EXTRA_BLOCKS = Array.from(
  { length: 96 },
  (_, index) => `${39 + Math.floor(index / 16)}.${(index % 16) * 16}.0.0/12`,
);

const IP_ENTRIES: Record<string, GeoEntry[]> = {
  cloudflare: [
    '103.21.244.0/22',
    '103.22.200.0/22',
    '103.31.4.0/22',
    '104.16.0.0/13',
    '104.24.0.0/14',
    '108.162.192.0/18',
    '131.0.72.0/22',
    '141.101.64.0/18',
    '162.158.0.0/15',
    '172.64.0.0/13',
    '173.245.48.0/20',
    '188.114.96.0/20',
    '190.93.240.0/20',
    '197.234.240.0/22',
    '198.41.128.0/17',
    '2400:cb00::/32',
    '2606:4700::/32',
  ].map(cidr),
  cn: [...CN_BLOCKS, ...CN_EXTRA_BLOCKS].map(cidr),
  facebook: [
    '31.13.24.0/21',
    '31.13.64.0/18',
    '66.220.144.0/20',
    '69.63.176.0/20',
    '69.171.224.0/19',
    '157.240.0.0/16',
    '179.60.192.0/22',
    '185.60.216.0/22',
    '2a03:2880::/32',
  ].map(cidr),
  google: [
    '8.8.4.0/24',
    '8.8.8.0/24',
    '34.64.0.0/10',
    '35.184.0.0/13',
    '64.233.160.0/19',
    '66.102.0.0/20',
    '72.14.192.0/18',
    '74.125.0.0/16',
    '108.177.8.0/21',
    '142.250.0.0/15',
    '172.217.0.0/16',
    '216.58.192.0/19',
    '2404:6800::/32',
    '2607:f8b0::/32',
  ].map(cidr),
  ir: [
    '2.144.0.0/14',
    '5.22.0.0/17',
    '31.2.128.0/17',
    '37.32.0.0/19',
    '46.32.0.0/19',
    '78.38.0.0/15',
    '80.191.0.0/16',
    '85.15.0.0/18',
    '91.98.0.0/15',
    '178.22.72.0/21',
    '185.8.172.0/22',
    '188.34.0.0/17',
    '217.218.0.0/15',
  ].map(cidr),
  netflix: [
    '23.246.0.0/18',
    '37.77.184.0/21',
    '45.57.0.0/17',
    '64.120.128.0/17',
    '66.197.128.0/17',
    '108.175.32.0/20',
    '185.2.220.0/22',
    '192.173.64.0/18',
    '198.38.96.0/19',
    '198.45.48.0/20',
  ].map(cidr),
  private: [
    '0.0.0.0/8',
    '10.0.0.0/8',
    '100.64.0.0/10',
    '127.0.0.0/8',
    '169.254.0.0/16',
    '172.16.0.0/12',
    '192.0.0.0/24',
    '192.0.2.0/24',
    '192.168.0.0/16',
    '198.18.0.0/15',
    '198.51.100.0/24',
    '203.0.113.0/24',
    '224.0.0.0/4',
    '240.0.0.0/4',
    '255.255.255.255/32',
    '::1/128',
    'fc00::/7',
    'fe80::/10',
  ].map(cidr),
  ru: [
    '2.60.0.0/14',
    '5.8.0.0/19',
    '31.6.0.0/17',
    '37.9.0.0/19',
    '46.16.0.0/21',
    '62.76.0.0/18',
    '77.37.128.0/17',
    '78.24.216.0/21',
    '79.104.0.0/15',
    '80.64.128.0/19',
    '81.16.96.0/19',
    '82.140.128.0/18',
    '85.113.0.0/16',
    '87.226.0.0/16',
    '91.77.0.0/16',
    '93.157.0.0/17',
    '94.19.0.0/16',
    '95.24.0.0/13',
    '178.176.0.0/13',
    '188.128.0.0/13',
    '213.87.0.0/16',
    '217.66.152.0/21',
    '2a00:1148::/32',
  ].map(cidr),
  telegram: [
    '91.108.4.0/22',
    '91.108.8.0/22',
    '91.108.12.0/22',
    '91.108.16.0/22',
    '91.108.20.0/22',
    '91.108.56.0/22',
    '149.154.160.0/20',
    '2001:67c:4e8::/48',
    '2001:b28:f23d::/48',
    '2001:b28:f23f::/48',
  ].map(cidr),
  us: [
    '3.0.0.0/9',
    '12.0.0.0/8',
    '23.192.0.0/11',
    '34.192.0.0/10',
    '50.16.0.0/14',
    '52.0.0.0/10',
    '63.64.0.0/11',
    '65.0.0.0/10',
    '68.32.0.0/11',
    '71.0.0.0/11',
    '96.0.0.0/9',
    '128.0.0.0/10',
    '199.0.0.0/12',
    '208.64.0.0/12',
    '2600:1f00::/24',
  ].map(cidr),
};

function categoriesOf(
  entries: Record<string, GeoEntry[]>,
  attributes: Record<string, string[]> = {},
): GeoCategory[] {
  return Object.keys(entries)
    .sort()
    .map((code) => ({ code, entries: entries[code].length, attributes: attributes[code] ?? [] }));
}

const SITE_CATEGORIES = categoriesOf(SITE_ENTRIES, SITE_ATTRIBUTES);
const IP_CATEGORIES = categoriesOf(IP_ENTRIES);

const UPDATED_AT = Date.UTC(2026, 6, 24, 3, 12);

const GEOSITE_FILE: GeoFile = {
  name: 'geosite.dat',
  kind: 'site',
  size: 4_812_544,
  modifiedAt: UPDATED_AT,
  categories: SITE_CATEGORIES.length,
};

const GEOIP_FILE: GeoFile = {
  name: 'geoip.dat',
  kind: 'ip',
  size: 8_694_272,
  modifiedAt: UPDATED_AT,
  categories: IP_CATEGORIES.length,
};

const DAMAGED_FILE: GeoFile = {
  name: 'geosite-custom.dat',
  kind: 'site',
  size: 262_144,
  modifiedAt: Date.UTC(2026, 5, 2, 19, 45),
  categories: 0,
  error: 'proto: cannot parse invalid wire-format data',
};

const OVERSIZED_FILE: GeoFile = {
  name: 'geoip-full.dat',
  kind: 'ip',
  size: 96_468_992,
  modifiedAt: Date.UTC(2026, 6, 20, 8, 5),
  categories: 0,
  error: 'geodata file is too large to browse',
};

const DATASETS: Record<string, { categories: GeoCategory[]; entries: Record<string, GeoEntry[]> }> =
  {
    'geosite.dat': { categories: SITE_CATEGORIES, entries: SITE_ENTRIES },
    'geoip.dat': { categories: IP_CATEGORIES, entries: IP_ENTRIES },
  };

function routesFor(files: GeoFile[]): GeoRoutes {
  return {
    '/panel/api/xray/geodata/files': () => files,
    '/panel/api/xray/geodata/categories': (query) => {
      const dataset = DATASETS[query.get('file') ?? ''];
      const needle = (query.get('q') ?? '').trim().toLowerCase();
      const items = (dataset?.categories ?? []).filter((category) =>
        category.code.includes(needle),
      );
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
  };
}

function withFiles(files: GeoFile[]): Decorator {
  const routes = routesFor(files);
  return function GeodataBackend(Story) {
    return (
      <GeoApi routes={routes}>
        <Story />
      </GeoApi>
    );
  };
}

const withDatabases = withFiles([GEOSITE_FILE, GEOIP_FILE]);

function BrowserDemo(props: GeoBrowserModalProps) {
  const [open, setOpen] = useState(props.open);
  const [value, setValue] = useState(props.value);
  useEffect(() => setOpen(props.open), [props.open]);
  useEffect(() => setValue(props.value), [props.value]);
  return (
    <Space direction="vertical" size={12}>
      <Space size={8}>
        <Button onClick={() => setOpen(true)}>Open geo browser</Button>
        <Typography.Text code>{value || 'no rule yet'}</Typography.Text>
      </Space>
      <GeoBrowserModal
        {...props}
        open={open}
        value={value}
        onApply={(next) => {
          setValue(next);
          setOpen(false);
        }}
        onClose={() => setOpen(false)}
      />
    </Space>
  );
}

const meta = {
  title: 'Geodata/GeoBrowserModal',
  component: GeoBrowserModal,
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
          'Browser for the geosite/geoip `.dat` databases Xray resolves `geosite:` and `geoip:` routing tokens against: pick a database, search its categories, tick the ones a rule needs, and preview the domains or CIDRs inside the highlighted category. Applying merges the ticked categories back into the rule string, keeping hand-typed domains untouched. The stories serve `/panel/api/xray/geodata/*` from an in-memory fixture, so search, paging and selection all work without a panel backend.',
      },
    },
  },
  args: {
    open: true,
    kind: 'site',
    value: '',
    onApply: () => undefined,
    onClose: () => undefined,
  },
  argTypes: {
    open: { description: 'Whether the modal is visible.' },
    kind: {
      description:
        'Which database layout the rule targets: `site` for domain rules, `ip` for CIDR rules. Decides the preselected database and the token prefix.',
      control: 'inline-radio',
      options: ['site', 'ip'],
    },
    value: {
      description:
        'Current rule string, comma separated. Tokens that match a category in the opened database come back preselected.',
    },
    onApply: { description: 'Called with the merged rule string when Apply is pressed.' },
    onClose: { description: 'Called when the modal is dismissed.' },
  },
  render: (args) => <BrowserDemo {...args} />,
} satisfies Meta<typeof GeoBrowserModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const SiteDatabase: Story = {
  decorators: [withDatabases],
  args: { kind: 'site', value: 'geosite:google, geosite:telegram, ads.example.com' },
};

export const CategoryPreview: Story = {
  decorators: [withDatabases],
  args: { kind: 'site', value: 'geosite:google' },
  parameters: {
    a11y: {
      config: {
        rules: [
          { id: 'color-contrast', enabled: false },
          { id: 'scrollable-region-focusable', enabled: false },
        ],
      },
    },
  },
  play: async ({ canvasElement, userEvent }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.type(await body.findByPlaceholderText('Search category'), 'telegram');
    await userEvent.click(await body.findByText('telegram'));
    await expect(await body.findByText('t.me')).toBeVisible();
  },
};

export const IpDatabase: Story = {
  decorators: [withDatabases],
  args: { kind: 'ip', value: 'geoip:private, 10.0.0.0/8' },
};

export const NoDatabases: Story = {
  decorators: [withFiles([])],
  args: { kind: 'site', value: 'geosite:google' },
};

export const DamagedDatabase: Story = {
  decorators: [withFiles([GEOSITE_FILE, DAMAGED_FILE, OVERSIZED_FILE])],
  args: { kind: 'site', value: '' },
};
