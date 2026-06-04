import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, InputNumber, Modal, Select, Space, Switch, Tabs } from 'antd';
import {
  BarChartOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  ReloadOutlined,
  SettingOutlined,
} from '@ant-design/icons';

import { OutboundDomainStrategies } from '@/schemas/primitives';
import { SettingListItem } from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from '@/pages/settings/catTabLabel';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import './BasicsTab.css';

import {
  ACCESS_LOG,
  ERROR_LOG,
  LOG_LEVELS,
  MASK_ADDRESS,
  ROUTING_DOMAIN_STRATEGIES,
} from './constants';

interface BasicsTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  outboundTestUrl: string;
  onChangeOutboundTestUrl: (v: string) => void;
  onResetDefault: () => void;
}

export default function BasicsTab({
  templateSettings,
  setTemplateSettings,
  outboundTestUrl,
  onChangeOutboundTestUrl,
  onResetDefault,
}: BasicsTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
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

  const setLevel0 = useCallback(
    (field: string, value: number | null) => mutate((tt) => {
      if (!tt.policy) tt.policy = {};
      if (!tt.policy.levels) tt.policy.levels = {};
      if (!tt.policy.levels['0']) tt.policy.levels['0'] = {};
      if (value === null || value === undefined) {
        delete tt.policy.levels['0'][field];
      } else {
        tt.policy.levels['0'][field] = value;
      }
    }),
    [mutate],
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
  const level0 = (templateSettings?.policy?.levels?.['0'] || {}) as Record<string, unknown>;

  const items = [
    {
      key: '1',
      label: catTabLabel(<SettingOutlined />, t('pages.xray.generalConfigs'), isMobile),
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
      label: catTabLabel(<BarChartOutlined />, t('pages.xray.statistics'), isMobile),
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
      key: 'connection',
      label: catTabLabel(<ClockCircleOutlined />, t('pages.xray.connectionLimits'), isMobile),
      children: (
        <>
          <Alert
            type="warning"
            showIcon
            className="mb-12 hint-alert"
            title={t('pages.xray.connectionLimitsDesc')}
          />
          <SettingListItem
            title={t('pages.xray.connIdle')}
            description={t('pages.xray.connIdleDesc')}
            paddings="small"
            control={
              <InputNumber
                value={typeof level0.connIdle === 'number' ? level0.connIdle : undefined}
                min={0}
                style={{ width: '100%' }}
                placeholder="300"
                addonAfter={t('pages.xray.seconds')}
                onChange={(v) => setLevel0('connIdle', v as number | null)}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.bufferSize')}
            description={t('pages.xray.bufferSizeDesc')}
            paddings="small"
            control={
              <InputNumber
                value={typeof level0.bufferSize === 'number' ? level0.bufferSize : undefined}
                min={0}
                style={{ width: '100%' }}
                placeholder={t('pages.xray.bufferSizePlaceholder')}
                addonAfter="KB"
                onChange={(v) => setLevel0('bufferSize', v as number | null)}
              />
            }
          />
        </>
      ),
    },
    {
      key: '3',
      label: catTabLabel(<FileTextOutlined />, t('pages.xray.logConfigs'), isMobile),
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
      key: 'reset',
      label: catTabLabel(<ReloadOutlined />, t('pages.settings.resetDefaultConfig'), isMobile),
      children: (
        <Space style={{ padding: '0 20px' }}>
          <Button type="primary" danger icon={<ReloadOutlined />} onClick={confirmResetDefault}>
            {t('pages.settings.resetDefaultConfig')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <>
      {modalContextHolder}
      <Tabs defaultActiveKey="1" items={items} />
    </>
  );
}
