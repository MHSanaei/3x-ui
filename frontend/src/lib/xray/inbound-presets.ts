import type { FormInstance } from 'antd';

import { RandomUtil } from '@/utils';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';
import { HysteriaStreamSettingsSchema } from '@/schemas/protocols/stream/hysteria';
import { XHttpStreamSettingsSchema } from '@/schemas/protocols/stream/xhttp';
import { TcpStreamSettingsSchema } from '@/schemas/protocols/stream/tcp';
import { RealityStreamSettingsSchema } from '@/schemas/protocols/security/reality';
import { createHysteriaTlsSettingsWithDefaultCert } from '@/lib/xray/inbound-tls-defaults';

export type InboundPresetKey =
  | 'iran-tcp-resilient'
  | 'iran-http-like'
  | 'iran-udp-fast';

export const INBOUND_PRESETS: Array<{ key: InboundPresetKey; label: string; description: string }> = [
  {
    key: 'iran-tcp-resilient',
    label: 'Iran TCP resilient',
    description: 'TCP + Reality with diverse shortIds, browser fingerprint, and non-root spiderX.',
  },
  {
    key: 'iran-http-like',
    label: 'Iran HTTP-like',
    description: 'XHTTP + Reality with a plausible web path, session IDs, and client-propagated HTTP knobs.',
  },
  {
    key: 'iran-udp-fast',
    label: 'Iran UDP fast',
    description: 'Hysteria2 + TLS, masquerade, Salamander/Gecko, and UDP port hopping.',
  },
];

const REALITY_TARGETS = [
  'www.microsoft.com',
  'www.samsung.com',
  'www.gstatic.com',
  'www.nvidia.com',
  'www.intel.com',
  'dl.google.com',
] as const;

const FINGERPRINTS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'randomized'] as const;

function pick<T>(items: readonly T[]): T {
  return items[RandomUtil.randomInteger(0, items.length - 1)];
}

function randomWebPath(prefix: string): string {
  return `/${prefix}/${RandomUtil.randomLowerAndNum(10)}`;
}

function randomRealitySettings(domain = pick(REALITY_TARGETS)) {
  const reality = RealityStreamSettingsSchema.parse({});
  return {
    ...reality,
    target: `${domain}:443`,
    serverNames: [domain],
    shortIds: RandomUtil.randomShortIds().split(',').map((s) => s.trim()).filter(Boolean),
    settings: {
      ...reality.settings,
      fingerprint: pick(FINGERPRINTS),
      serverName: domain,
      spiderX: randomWebPath('assets'),
    },
  };
}

function randomFinalMaskPassword(): string {
  return RandomUtil.randomLowerAndNum(20);
}

export function applyInboundPreset(form: FormInstance<InboundFormValues>, key: InboundPresetKey): void {
  const current = (form.getFieldValue('streamSettings') ?? {}) as Record<string, unknown>;

  if (key === 'iran-tcp-resilient') {
    form.setFieldValue('port', 443);
    form.setFieldValue('streamSettings', {
      ...current,
      network: 'tcp',
      security: 'reality',
      tcpSettings: TcpStreamSettingsSchema.parse({ header: { type: 'none' } }),
      realitySettings: randomRealitySettings(),
    });
    return;
  }

  if (key === 'iran-http-like') {
    const domain = pick(REALITY_TARGETS);
    form.setFieldValue('port', 443);
    form.setFieldValue('streamSettings', {
      ...current,
      network: 'xhttp',
      security: 'reality',
      xhttpSettings: XHttpStreamSettingsSchema.parse({
        host: domain,
        path: randomWebPath('api'),
        mode: 'auto',
        sessionIDPlacement: 'path',
        sessionIDTable: 'Base62',
        sessionIDLength: '8-16',
        scMinPostsIntervalMs: '50-150',
        headers: {
          accept: 'application/json, text/plain, */*',
        },
      }),
      realitySettings: randomRealitySettings(domain),
    });
    return;
  }

  if (key === 'iran-udp-fast') {
    form.setFieldValue('port', 443);
    form.setFieldValue('streamSettings', {
      ...current,
      network: 'hysteria',
      security: 'tls',
      hysteriaSettings: HysteriaStreamSettingsSchema.parse({
        udpIdleTimeout: 90,
        masquerade: {
          type: 'string',
          statusCode: 200,
          content: 'ok',
          headers: { 'content-type': 'text/plain; charset=utf-8' },
        },
      }),
      tlsSettings: createHysteriaTlsSettingsWithDefaultCert(),
      finalmask: {
        udp: [{
          type: 'salamander',
          settings: {
            password: randomFinalMaskPassword(),
            packetSize: '512-1200',
          },
        }],
        quicParams: {
          congestion: 'bbr',
          bbrProfile: 'standard',
          udpHop: { ports: '20000-50000', interval: '5-10' },
          maxIdleTimeout: 30,
          keepAlivePeriod: 10,
        },
      },
    });
  }
}
