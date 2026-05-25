import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Collapse, Input, Modal, Select, Space, Switch } from 'antd';
import { CloudOutlined, ApiOutlined } from '@ant-design/icons';

import { OutboundDomainStrategies } from '@/models/outbound';
import SettingListItem from '@/components/SettingListItem';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import './BasicsTab.css';

interface BasicsTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  outboundTestUrl: string;
  onChangeOutboundTestUrl: (v: string) => void;
  warpExist: boolean;
  nordExist: boolean;
  onShowWarp: () => void;
  onShowNord: () => void;
  onResetDefault: () => void;
}

const ROUTING_DOMAIN_STRATEGIES = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'];
const LOG_LEVELS = ['none', 'debug', 'info', 'warning', 'error'];
const ACCESS_LOG = ['none', './access.log'];
const ERROR_LOG = ['none', './error.log'];
const MASK_ADDRESS = ['quarter', 'half', 'full'];
const BITTORRENT_PROTOCOLS = ['bittorrent'];

const IPS_OPTIONS = [
  { label: 'Private IPs', value: 'geoip:private' },
  { label: '🇮🇷 Iran', value: 'ext:geoip_IR.dat:ir' },
  { label: '🇨🇳 China', value: 'geoip:cn' },
  { label: '🇷🇺 Russia', value: 'ext:geoip_RU.dat:ru' },
  { label: '🇻🇳 Vietnam', value: 'geoip:vn' },
  { label: '🇪🇸 Spain', value: 'geoip:es' },
  { label: '🇮🇩 Indonesia', value: 'geoip:id' },
  { label: '🇺🇦 Ukraine', value: 'geoip:ua' },
  { label: '🇹🇷 Türkiye', value: 'geoip:tr' },
  { label: '🇧🇷 Brazil', value: 'geoip:br' },
];
const DOMAINS_OPTIONS = [
  { label: '🇮🇷 Iran', value: 'ext:geosite_IR.dat:ir' },
  { label: '🇮🇷 .ir', value: 'regexp:.*\\.ir$' },
  { label: '🇮🇷 .ایران', value: 'regexp:.*\\.xn--mgba3a4f16a$' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇨🇳 .cn', value: 'regexp:.*\\.cn$' },
  { label: '🇷🇺 Russia', value: 'ext:geosite_RU.dat:ru-available-only-inside' },
  { label: '🇷🇺 .ru', value: 'regexp:.*\\.ru$' },
  { label: '🇷🇺 .su', value: 'regexp:.*\\.su$' },
  { label: '🇷🇺 .рф', value: 'regexp:.*\\.xn--p1ai$' },
  { label: '🇻🇳 .vn', value: 'regexp:.*\\.vn$' },
];
const BLOCK_DOMAINS_OPTIONS = [
  { label: 'Ads All', value: 'geosite:category-ads-all' },
  { label: 'Ads IR 🇮🇷', value: 'ext:geosite_IR.dat:category-ads-all' },
  { label: 'Ads RU 🇷🇺', value: 'ext:geosite_RU.dat:category-ads-all' },
  { label: 'Malware 🇮🇷', value: 'ext:geosite_IR.dat:malware' },
  { label: 'Phishing 🇮🇷', value: 'ext:geosite_IR.dat:phishing' },
  { label: 'Cryptominers 🇮🇷', value: 'ext:geosite_IR.dat:cryptominers' },
  { label: 'Adult +18', value: 'geosite:category-porn' },
  { label: '🇮🇷 Iran', value: 'ext:geosite_IR.dat:ir' },
  { label: '🇮🇷 .ir', value: 'regexp:.*\\.ir$' },
  { label: '🇮🇷 .ایران', value: 'regexp:.*\\.xn--mgba3a4f16a$' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇨🇳 .cn', value: 'regexp:.*\\.cn$' },
  { label: '🇷🇺 Russia', value: 'ext:geosite_RU.dat:ru-available-only-inside' },
  { label: '🇷🇺 .ru', value: 'regexp:.*\\.ru$' },
  { label: '🇷🇺 .su', value: 'regexp:.*\\.su$' },
  { label: '🇷🇺 .рф', value: 'regexp:.*\\.xn--p1ai$' },
  { label: '🇻🇳 .vn', value: 'regexp:.*\\.vn$' },
];
const SERVICES_OPTIONS = [
  { label: 'Apple', value: 'geosite:apple' },
  { label: 'Meta', value: 'geosite:meta' },
  { label: 'Google', value: 'geosite:google' },
  { label: 'OpenAI', value: 'geosite:openai' },
  { label: 'Spotify', value: 'geosite:spotify' },
  { label: 'Netflix', value: 'geosite:netflix' },
  { label: 'Reddit', value: 'geosite:reddit' },
  { label: 'Speedtest', value: 'geosite:speedtest' },
];

const directSettings = { tag: 'direct', protocol: 'freedom' };
const ipv4Settings = { tag: 'IPv4', protocol: 'freedom', settings: { domainStrategy: 'UseIPv4' } };

function ruleGetter(t: XraySettingsValue | null, outboundTag: string, property: string): string[] {
  if (!t?.routing?.rules) return [];
  const out: string[] = [];
  for (const rule of t.routing.rules) {
    if (
      rule &&
      Object.prototype.hasOwnProperty.call(rule, property) &&
      Object.prototype.hasOwnProperty.call(rule, 'outboundTag') &&
      rule.outboundTag === outboundTag
    ) {
      const v = (rule as Record<string, unknown>)[property];
      if (Array.isArray(v)) out.push(...(v as string[]));
    }
  }
  return out;
}

function ruleSetter(t: XraySettingsValue, outboundTag: string, property: string, data: string[]): void {
  if (!t.routing) return;
  if (!Array.isArray(t.routing.rules)) t.routing.rules = [];
  const current = ruleGetter(t, outboundTag, property);
  if (current.length === 0) {
    t.routing.rules.push({ type: 'field', outboundTag, [property]: data });
    return;
  }
  const next: typeof t.routing.rules = [];
  let inserted = false;
  for (const rule of t.routing.rules) {
    const matches =
      rule &&
      Object.prototype.hasOwnProperty.call(rule, property) &&
      Object.prototype.hasOwnProperty.call(rule, 'outboundTag') &&
      rule.outboundTag === outboundTag;
    if (matches) {
      if (!inserted && data.length > 0) {
        (rule as Record<string, unknown>)[property] = data;
        next.push(rule);
        inserted = true;
      }
    } else {
      next.push(rule);
    }
  }
  t.routing.rules = next;
}

function syncOutbound(t: XraySettingsValue, tag: string, settings: Record<string, unknown>) {
  if (!t.outbounds || !t.routing) return;
  const rules = t.routing.rules || [];
  const haveRules = rules.some((r) => r?.outboundTag === tag);
  const idx = t.outbounds.findIndex((o) => o?.tag === tag);
  if (!haveRules && idx > 0) t.outbounds.splice(idx, 1);
  if (haveRules && idx < 0) t.outbounds.push(settings as never);
}

export default function BasicsTab({
  templateSettings,
  setTemplateSettings,
  outboundTestUrl,
  onChangeOutboundTestUrl,
  warpExist,
  nordExist,
  onShowWarp,
  onShowNord,
  onResetDefault,
}: BasicsTabProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();

  const mutate = useCallback(
    (mutator: (next: XraySettingsValue) => void) => {
      setTemplateSettings((prev) => {
        if (!prev) return prev;
        const clone = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
        mutator(clone);
        return clone;
      });
    },
    [setTemplateSettings],
  );

  function confirmResetDefault() {
    modal.confirm({
      title: t('pages.settings.resetDefaultConfig'),
      okText: t('reset'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: () => onResetDefault(),
    });
  }

  const freedomStrategy =
    (templateSettings?.outbounds?.find((o) => o?.protocol === 'freedom' && o?.tag === 'direct')?.settings as
      | { domainStrategy?: string }
      | undefined)?.domainStrategy ?? 'AsIs';

  const routingStrategy = templateSettings?.routing?.domainStrategy ?? 'AsIs';
  const log = (templateSettings?.log || {}) as Record<string, unknown>;
  const policy = (templateSettings?.policy?.system || {}) as Record<string, boolean>;

  const blockedIPs = ruleGetter(templateSettings, 'blocked', 'ip');
  const blockedDomains = ruleGetter(templateSettings, 'blocked', 'domain');
  const blockedProtocols = ruleGetter(templateSettings, 'blocked', 'protocol');
  const directIPs = ruleGetter(templateSettings, 'direct', 'ip');
  const directDomains = ruleGetter(templateSettings, 'direct', 'domain');
  const ipv4Domains = ruleGetter(templateSettings, 'IPv4', 'domain');
  const warpDomains = ruleGetter(templateSettings, 'warp', 'domain');
  const nordTag =
    templateSettings?.outbounds?.find((o) => o?.tag?.startsWith?.('nord-'))?.tag || 'nord';
  const nordDomains = ruleGetter(templateSettings, nordTag, 'domain');

  const torrentActive = BITTORRENT_PROTOCOLS.every((p) => blockedProtocols.includes(p));

  const items = [
    {
      key: '1',
      label: t('pages.xray.generalConfigs'),
      children: (
        <>
          <Alert
            type="warning"
            showIcon
            className="mb-12 hint-alert"
            title={t('pages.xray.generalConfigsDesc')}
          />
          <SettingListItem
            title={t('pages.xray.FreedomStrategy')}
            description={t('pages.xray.FreedomStrategyDesc')}
            paddings="small"
            control={
              <Select
                value={freedomStrategy}
                style={{ width: '100%' }}
                options={(OutboundDomainStrategies as string[]).map((s) => ({ value: s, label: s }))}
                onChange={(next) => mutate((tt) => {
                  if (!tt.outbounds) tt.outbounds = [];
                  const idx = tt.outbounds.findIndex((o) => o?.protocol === 'freedom' && o?.tag === 'direct');
                  if (idx < 0) {
                    tt.outbounds.push({ protocol: 'freedom', tag: 'direct', settings: { domainStrategy: next } });
                  } else {
                    const ob = tt.outbounds[idx];
                    ob.settings = (ob.settings || {}) as Record<string, unknown>;
                    (ob.settings as Record<string, unknown>).domainStrategy = next;
                  }
                })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.RoutingStrategy')}
            description={t('pages.xray.RoutingStrategyDesc')}
            paddings="small"
            control={
              <Select
                value={routingStrategy}
                style={{ width: '100%' }}
                options={ROUTING_DOMAIN_STRATEGIES.map((s) => ({ value: s, label: s }))}
                onChange={(next) => mutate((tt) => {
                  if (tt.routing) tt.routing.domainStrategy = next;
                })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.outboundTestUrl')}
            description={t('pages.xray.outboundTestUrlDesc')}
            paddings="small"
            control={
              <Input
                value={outboundTestUrl}
                onChange={(e) => onChangeOutboundTestUrl(e.target.value)}
                placeholder="https://www.google.com/generate_204"
              />
            }
          />
        </>
      ),
    },
    {
      key: '2',
      label: t('pages.xray.statistics'),
      children: (
        <>
          {[
            ['statsInboundUplink', t('pages.xray.statsInboundUplink')],
            ['statsInboundDownlink', t('pages.xray.statsInboundDownlink')],
            ['statsOutboundUplink', t('pages.xray.statsOutboundUplink')],
            ['statsOutboundDownlink', 'Outbound downlink stats'],
          ].map(([field, label]) => (
            <SettingListItem
              key={field}
              title={label}
              paddings="small"
              control={
                <Switch
                  checked={!!policy[field]}
                  onChange={(checked) => mutate((tt) => {
                    if (!tt.policy) tt.policy = {};
                    if (!tt.policy.system) tt.policy.system = {};
                    tt.policy.system[field] = checked;
                  })}
                />
              }
            />
          ))}
        </>
      ),
    },
    {
      key: '3',
      label: t('pages.xray.logConfigs'),
      children: (
        <>
          <Alert
            type="warning"
            showIcon
            className="mb-12 hint-alert"
            title={t('pages.xray.logConfigsDesc')}
          />
          <SettingListItem
            title={t('pages.xray.logLevel')}
            description={t('pages.xray.logLevelDesc')}
            paddings="small"
            control={
              <Select
                value={(log.loglevel as string) || 'warning'}
                style={{ width: '100%' }}
                options={LOG_LEVELS.map((s) => ({ value: s, label: s }))}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.loglevel = v; })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.accessLog')}
            description={t('pages.xray.accessLogDesc')}
            paddings="small"
            control={
              <Select
                value={(log.access as string) || ''}
                style={{ width: '100%' }}
                options={ACCESS_LOG.map((s) => ({ value: s, label: s }))}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.access = v; })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.errorLog')}
            description={t('pages.xray.errorLogDesc')}
            paddings="small"
            control={
              <Select
                value={(log.error as string) || ''}
                style={{ width: '100%' }}
                options={[{ value: '', label: t('empty') }, ...ERROR_LOG.map((s) => ({ value: s, label: s }))]}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.error = v; })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.maskAddress')}
            description={t('pages.xray.maskAddressDesc')}
            paddings="small"
            control={
              <Select
                value={(log.maskAddress as string) || ''}
                style={{ width: '100%' }}
                options={[{ value: '', label: t('empty') }, ...MASK_ADDRESS.map((s) => ({ value: s, label: s }))]}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.maskAddress = v; })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.dnsLog')}
            description={t('pages.xray.dnsLogDesc')}
            paddings="small"
            control={
              <Switch
                checked={!!log.dnsLog}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.dnsLog = v; })}
              />
            }
          />
        </>
      ),
    },
    {
      key: '4',
      label: t('pages.xray.basicRouting'),
      children: (
        <>
          <Alert
            type="warning"
            showIcon
            className="mb-12 hint-alert"
            title={t('pages.xray.blockConnectionsConfigsDesc')}
          />

          <SettingListItem
            title={t('pages.xray.Torrent')}
            paddings="small"
            control={
              <Switch
                checked={torrentActive}
                onChange={(checked) => mutate((tt) => {
                  const next = checked
                    ? [...blockedProtocols, ...BITTORRENT_PROTOCOLS]
                    : blockedProtocols.filter((d) => !BITTORRENT_PROTOCOLS.includes(d));
                  ruleSetter(tt, 'blocked', 'protocol', next);
                })}
              />
            }
          />

          <SettingListItem
            title={t('pages.xray.blockips')}
            paddings="small"
            control={
              <Select
                mode="tags"
                value={blockedIPs}
                style={{ width: '100%' }}
                options={IPS_OPTIONS}
                onChange={(v) => mutate((tt) => ruleSetter(tt, 'blocked', 'ip', v))}
              />
            }
          />

          <SettingListItem
            title={t('pages.xray.blockdomains')}
            paddings="small"
            control={
              <Select
                mode="tags"
                value={blockedDomains}
                style={{ width: '100%' }}
                options={BLOCK_DOMAINS_OPTIONS}
                onChange={(v) => mutate((tt) => ruleSetter(tt, 'blocked', 'domain', v))}
              />
            }
          />

          <Alert
            type="warning"
            showIcon
            className="mb-12 hint-alert"
            title={t('pages.xray.directConnectionsConfigsDesc')}
          />

          <SettingListItem
            title={t('pages.xray.directips')}
            paddings="small"
            control={
              <Select
                mode="tags"
                value={directIPs}
                style={{ width: '100%' }}
                options={IPS_OPTIONS}
                onChange={(v) => mutate((tt) => {
                  ruleSetter(tt, 'direct', 'ip', v);
                  syncOutbound(tt, 'direct', directSettings);
                })}
              />
            }
          />

          <SettingListItem
            title={t('pages.xray.directdomains')}
            paddings="small"
            control={
              <Select
                mode="tags"
                value={directDomains}
                style={{ width: '100%' }}
                options={DOMAINS_OPTIONS}
                onChange={(v) => mutate((tt) => {
                  ruleSetter(tt, 'direct', 'domain', v);
                  syncOutbound(tt, 'direct', directSettings);
                })}
              />
            }
          />

          <SettingListItem
            title={t('pages.xray.ipv4Routing')}
            description={t('pages.xray.ipv4RoutingDesc')}
            paddings="small"
            control={
              <Select
                mode="tags"
                value={ipv4Domains}
                style={{ width: '100%' }}
                options={SERVICES_OPTIONS}
                onChange={(v) => mutate((tt) => {
                  ruleSetter(tt, 'IPv4', 'domain', v);
                  syncOutbound(tt, 'IPv4', ipv4Settings);
                })}
              />
            }
          />

          <SettingListItem
            title={t('pages.xray.warpRouting')}
            description={t('pages.xray.warpRoutingDesc')}
            paddings="small"
            control={
              warpExist ? (
                <Select
                  mode="tags"
                  value={warpDomains}
                  style={{ width: '100%' }}
                  options={SERVICES_OPTIONS}
                  onChange={(v) => mutate((tt) => ruleSetter(tt, 'warp', 'domain', v))}
                />
              ) : (
                <Button type="primary" onClick={onShowWarp} icon={<CloudOutlined />}>
                  WARP
                </Button>
              )
            }
          />

          <SettingListItem
            title={t('pages.xray.nordRouting')}
            description={t('pages.xray.nordRoutingDesc')}
            paddings="small"
            control={
              nordExist ? (
                <Select
                  mode="tags"
                  value={nordDomains}
                  style={{ width: '100%' }}
                  options={SERVICES_OPTIONS}
                  onChange={(v) => mutate((tt) => ruleSetter(tt, nordTag, 'domain', v))}
                />
              ) : (
                <Button type="primary" onClick={onShowNord} icon={<ApiOutlined />}>
                  NordVPN
                </Button>
              )
            }
          />
        </>
      ),
    },
    {
      key: 'reset',
      label: t('pages.settings.resetDefaultConfig'),
      children: (
        <Space style={{ padding: '0 20px' }}>
          <Button danger onClick={confirmResetDefault}>
            {t('pages.settings.resetDefaultConfig')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <>
      {modalContextHolder}
      <Collapse defaultActiveKey={['1']} items={items} />
    </>
  );
}
