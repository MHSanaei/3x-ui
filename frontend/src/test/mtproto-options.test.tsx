import type { ReactNode } from 'react';
import { Form } from 'antd';
import { fireEvent, screen } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { genAllLinks, genInboundLinks, mtprotoWebSecret } from '@/lib/xray/inbound-link';
import { parseLinkParts } from '@/lib/xray/link-label';
import MtprotoFields from '@/pages/inbounds/form/protocols/mtproto';
import { InboundSchema } from '@/schemas/api/inbound';
import { MtprotoInboundSettingsSchema } from '@/schemas/protocols/inbound/mtproto';
import { HttpUtil, Msg } from '@/utils';
import { fieldLabels, renderWithProviders } from './test-utils';

const SECRET = 'ee0123456789abcdef0123456789abcdef7777772e636c6f7564666c6172652e636f6d';
const KEY = '0123456789abcdef0123456789abcdef';

function parse(settings: Record<string, unknown>) {
  return MtprotoInboundSettingsSchema.safeParse({ clients: [], ...settings });
}

describe('mtproto mtg options schema', () => {
  it('keeps every option the backend renders', () => {
    const settings = {
      concurrency: 2048,
      tolerateTimeSkewness: '5s',
      allowFallbackOnUnknownDc: true,
      autoUpdate: true,
      throttleCheckInterval: '10s',
      network: {
        dns: 'https://1.1.1.1/dns-query',
        proxies: ['socks5://10.0.0.1:1080'],
        tcpNotSentLowat: '1mib',
        clientMss: 92,
        clientMssBulk: 1400,
        timeout: { tcp: '5s', http: '10s', idle: '5m', handshake: '10s' },
        keepAlive: { disabled: true, idle: '15s', interval: '15s', count: 9 },
      },
      defense: {
        antiReplay: { enabled: true, maxSize: '1mib', errorRate: 0.001 },
        blocklist: { enabled: true, urls: ['https://example.com/l.netset'], updateEach: '24h' },
        allowlist: { enabled: false, downloadConcurrency: 2 },
        doppelganger: { urls: ['https://cdn.example.com/a.js'], repeatsPerRaid: 10, drs: true },
        pendingHandshakes: { maxPerIp: 32, dryRun: true },
      },
      dcPool: { enabled: true, size: 4, dcs: [1, 2, -2, 203] },
      stats: {
        prometheus: { enabled: true, bindTo: '127.0.0.1:3129', httpPath: '/metrics' },
        statsd: {
          enabled: true,
          address: '10.0.0.5:8125',
          metricPrefix: 'mtg',
          tagFormat: 'influxdb',
        },
      },
      web: {
        bindTo: '127.0.0.1:18080',
        host: 'proxy.example.com',
        secretMode: 'plain',
        decoyDir: '/var/www/decoy',
        trustedProxies: ['127.0.0.1/32', '::1/128'],
        maxSessions: 1024,
        maxPending: 4096,
        diag: true,
      },
      extraToml: 'usage-state-file = "/var/lib/mtg/usage.json"\n',
    };
    const parsed = parse(settings);
    expect(parsed.success).toBe(true);
    expect(parsed.data).toMatchObject(settings);
  });

  it('treats cleared inputs as unset and coerces DC tags to numbers', () => {
    const parsed = parse({
      concurrency: null,
      tolerateTimeSkewness: '',
      network: { proxies: [], timeout: { idle: '' } },
      dcPool: { enabled: true, dcs: ['2', '-4', '203'] },
      web: { bindTo: '', host: '' },
      extraToml: '',
    });
    expect(parsed.success).toBe(true);
    expect(parsed.data?.concurrency).toBeUndefined();
    expect(parsed.data?.tolerateTimeSkewness).toBeUndefined();
    expect(parsed.data?.network?.proxies).toBeUndefined();
    expect(parsed.data?.network?.timeout?.idle).toBeUndefined();
    expect(parsed.data?.dcPool?.dcs).toEqual([2, -4, 203]);
    expect(parsed.data?.web?.bindTo).toBeUndefined();
    expect(parsed.data?.extraToml).toBeUndefined();
  });

  it('keeps an explicit session MSS of 0 apart from a cleared one', () => {
    const whole = parse({ network: { clientMss: 120, clientMssBulk: 0 } });
    expect(whole.data?.network?.clientMssBulk).toBe(0);
    const cleared = parse({ network: { clientMss: 92, clientMssBulk: null } });
    expect(cleared.success).toBe(true);
    expect(cleared.data?.network?.clientMssBulk).toBeUndefined();
  });

  it.each([
    [
      'a session MSS not above the ServerHello MSS',
      { clientMss: 1400, clientMssBulk: 1400 },
      'clientMssBulk',
      'pages.inbounds.form.mtgClientMssBulkTooSmall',
    ],
    [
      'a ServerHello MSS the kernel refuses on the socket',
      { clientMss: 87, clientMssBulk: 0 },
      'clientMss',
      'pages.inbounds.form.mtgClientMssKernelMin',
    ],
  ])('flags %s on the right field', (_name, network, field, message) => {
    const parsed = parse({ network });
    expect(parsed.success).toBe(false);
    expect(parsed.error?.issues).toEqual([
      expect.objectContaining({ path: ['network', field], message }),
    ]);
  });

  it.each([
    ['concurrency out of range', { concurrency: 70000 }],
    ['ServerHello MSS below 48', { network: { clientMss: 47 } }],
    ['ServerHello MSS above 1460', { network: { clientMss: 1461 } }],
    ['session MSS below 536', { network: { clientMss: 92, clientMssBulk: 535 } }],
    ['session MSS above 65495', { network: { clientMss: 92, clientMssBulk: 65496 } }],
    ['duration without a unit', { tolerateTimeSkewness: '5' }],
    ['bad size', { network: { tcpNotSentLowat: 'lots' } }],
    ['non-socks proxy', { network: { proxies: ['http://10.0.0.1:3128'] } }],
    ['error rate of 100', { defense: { antiReplay: { errorRate: 100 } } }],
    ['plain-http doppelganger URL', { defense: { doppelganger: { urls: ['http://a.b/c'] } } }],
    ['zero DC', { dcPool: { dcs: [2, 0] } }],
    ['duplicate DC', { dcPool: { dcs: [2, 2] } }],
    ['public metrics listener', { stats: { prometheus: { bindTo: '0.0.0.0:3129' } } }],
    ['metric prefix with a dot', { stats: { statsd: { metricPrefix: 'mtg.' } } }],
    ['public WEB listener', { web: { bindTo: '0.0.0.0:18080' } }],
    ['WEB host with a path', { web: { host: 'proxy.example.com/x' } }],
    ['unknown WEB key mode', { web: { secretMode: 'ee' } }],
    ['relative decoy dir', { web: { decoyDir: 'www' } }],
  ])('rejects %s', (_name, settings) => {
    expect(parse(settings).success).toBe(false);
  });
});

function webInbound(web: Record<string, unknown> | undefined) {
  return InboundSchema.parse({
    id: 71,
    remark: 'mt-web',
    port: 8443,
    protocol: 'mtproto',
    settings: {
      fakeTlsDomain: 'www.cloudflare.com',
      ...(web ? { web } : {}),
      clients: [{ email: 'alice', secret: SECRET, enable: true }],
    },
  });
}

describe('mtproto WEB links', () => {
  it('derives the link key from the FakeTLS key by mode', () => {
    expect(mtprotoWebSecret(SECRET)).toBe(`dd${KEY}`);
    expect(mtprotoWebSecret(SECRET, 'dd')).toBe(`dd${KEY}`);
    expect(mtprotoWebSecret(SECRET, 'plain')).toBe(KEY);
    expect(mtprotoWebSecret(`dd${KEY}`, 'plain')).toBe('');
  });

  it('adds one tg://webproxy link to the WEB domain', () => {
    const inbound = webInbound({ bindTo: '127.0.0.1:18080', host: 'web.example.com' });
    const entries = genAllLinks({
      inbound,
      remark: 'mt-web',
      client: { secret: SECRET },
      fallbackHostname: 'mt.example.test',
    });
    expect(entries).toEqual([
      expect.objectContaining({ remark: 'mt-web' }),
      { remark: 'mt-web-web', link: `tg://webproxy?server=web.example.com&secret=dd${KEY}` },
    ]);
    expect(parseLinkParts(entries[1].link)?.security).toBe('WEB');
  });

  it('uses the bare key in plain mode', () => {
    const out = genInboundLinks({
      inbound: webInbound({
        bindTo: '127.0.0.1:18080',
        host: 'web.example.com',
        secretMode: 'plain',
      }),
      remark: 'mt-web',
      fallbackHostname: 'mt.example.test',
    });
    expect(out.split('\r\n')[1]).toBe(`tg://webproxy?server=web.example.com&secret=${KEY}`);
  });

  it.each([
    ['no WEB section', undefined],
    ['no listener', { host: 'web.example.com' }],
    ['no domain', { bindTo: '127.0.0.1:18080' }],
  ])('emits no WEB link with %s', (_name, web) => {
    const out = genInboundLinks({
      inbound: webInbound(web),
      remark: 'mt-web',
      fallbackHostname: 'mt.example.test',
    });
    expect(out).not.toContain('webproxy');
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

function Harness({
  children,
  settings,
}: {
  children: ReactNode;
  settings: Record<string, unknown>;
}) {
  const methods = useForm({ defaultValues: { settings } });
  return (
    <FormProvider {...methods}>
      <Form>{children}</Form>
    </FormProvider>
  );
}

function renderFields(settings: Record<string, unknown> = {}) {
  vi.spyOn(HttpUtil, 'post').mockResolvedValue(
    new Msg(true, '', JSON.stringify({ xraySetting: { outbounds: [] } })),
  );
  renderWithProviders(
    <Harness settings={settings}>
      <MtprotoFields />
    </Harness>,
  );
}

function openPanel(title: string) {
  fireEvent.click(screen.getByText(title));
}

describe('mtproto advanced settings form', () => {
  it('groups the mtg options into collapsed sections', () => {
    renderFields();
    for (const title of [
      'Relay',
      'Advanced: general',
      'Advanced: network',
      'Advanced: defense',
      'Advanced: metrics',
      'Advanced: WEB mode',
      'Advanced: extra TOML',
    ]) {
      expect(screen.getByText(title)).toBeTruthy();
    }
    expect(fieldLabels()).not.toContain('WEB listener');
  });

  it('shows the WEB fields when the section is opened', () => {
    renderFields();
    openPanel('Advanced: WEB mode');
    const labels = fieldLabels();
    for (const label of [
      'WEB listener',
      'WEB domain',
      'Link key mode',
      'Decoy site directory',
      'Trusted proxies',
      'Max WEB sessions',
      'Max unused bridge tokens',
      'Bridge diagnostics',
    ]) {
      expect(labels).toContain(label);
    }
  });

  it('shows the defense and metrics fields', () => {
    renderFields();
    openPanel('Advanced: defense');
    openPanel('Advanced: metrics');
    const labels = fieldLabels();
    for (const label of [
      'Anti-replay cache',
      'IP blocklist',
      'IP allowlist',
      'Doppelganger URLs',
      'Pending handshakes per IP',
      'Dry run',
      'Prometheus metrics',
      'Metrics address',
      'StatsD metrics',
      'Tag format',
    ]) {
      expect(labels).toContain(label);
    }
  });

  it('shows the ServerHello splitting fields in the relay section', () => {
    renderFields();
    expect(fieldLabels()).not.toContain('ServerHello MSS');
    openPanel('Relay');
    const labels = fieldLabels();
    expect(labels).toContain('ServerHello MSS');
    expect(labels).toContain('Session MSS');
  });

  it('hides upstream proxies while routing through Xray', () => {
    renderFields({ routeThroughXray: true });
    openPanel('Advanced: network');
    const labels = fieldLabels();
    expect(labels).toContain('DNS resolver');
    expect(labels).not.toContain('Upstream SOCKS5 proxies');
  });

  it('offers upstream proxies without Xray routing', () => {
    renderFields();
    openPanel('Advanced: network');
    expect(fieldLabels()).toContain('Upstream SOCKS5 proxies');
  });

  it('shows the DC list only for an enabled pool', () => {
    renderFields({ dcPool: { enabled: true } });
    expect(fieldLabels()).toContain('DCs to keep warm');
  });

  it('hides the DC list for a disabled pool', () => {
    renderFields();
    expect(fieldLabels()).not.toContain('DCs to keep warm');
  });
});
