import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Collapse, Input, Modal, Select, Space, Switch } from 'antd';
import { CloudOutlined, ApiOutlined } from '@ant-design/icons';

import { OutboundDomainStrategies } from '@/schemas/primitives';
import { SettingListItem } from '@/components/ui';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import './BasicsTab.css';

import {
  ACCESS_LOG,
  BITTORRENT_PROTOCOLS,
  BLOCK_DOMAINS_OPTIONS,
  DOMAINS_OPTIONS,
  ERROR_LOG,
  IPS_OPTIONS,
  LOG_LEVELS,
  MASK_ADDRESS,
  ROUTING_DOMAIN_STRATEGIES,
  SERVICES_OPTIONS,
  directSettings,
  ipv4Settings,
} from './constants';
import { ruleGetter, ruleSetter, syncOutbound } from './helpers';

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
                options={OutboundDomainStrategies.map((s) => ({ value: s, label: s }))}
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
            ['statsOutboundDownlink', t('pages.xray.statsOutboundDownlink')],
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
