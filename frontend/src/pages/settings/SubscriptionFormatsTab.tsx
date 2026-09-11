import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Input, InputNumber, Select, Switch, Tabs } from 'antd';
import {
  FileTextOutlined,
  NodeIndexOutlined,
  PartitionOutlined,
  RocketOutlined,
  SendOutlined,
  SettingOutlined,
  StopOutlined,
} from '@ant-design/icons';
import type { AllSetting } from '@/models/setting';
import { onNumber } from '@/utils/onNumber';
import { SettingListItem } from '@/components/ui';
import { GoRegexInput } from '@/components/form';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import { sanitizePath, normalizePath } from './uriPath';
import { remoteSourceBadge } from './subscriptionShared';
import SubJsonFinalMaskForm from './SubJsonFinalMaskForm';
import './SubscriptionFormatsTab.css';

interface SubscriptionFormatsTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

const DEFAULT_MUX = {
  enabled: true,
  concurrency: 8,
  xudpConcurrency: 16,
  xudpProxyUDP443: 'reject',
};

type SubJsonRule = { type: string; outboundTag: string; domain?: string[]; ip?: string[] };

const DEFAULT_DIRECT_RULES: SubJsonRule[] = [
  { type: 'field', outboundTag: 'direct', domain: ['geosite:category-ir'] },
  { type: 'field', outboundTag: 'direct', ip: ['geoip:private', 'geoip:ir'] },
];
const DEFAULT_BLOCK_RULES: SubJsonRule[] = [
  { type: 'field', outboundTag: 'block', domain: ['geosite:category-ads-all'] },
];
const BLOCK_IP_RULE: SubJsonRule = { type: 'field', outboundTag: 'block', ip: [] };

const directIPsOptions = [
  { label: 'Private IP', value: 'geoip:private' },
  { label: '🇮🇷 Iran', value: 'geoip:ir' },
  { label: '🇨🇳 China', value: 'geoip:cn' },
  { label: '🇷🇺 Russia', value: 'geoip:ru' },
  { label: '🇻🇳 Vietnam', value: 'geoip:vn' },
  { label: '🇪🇸 Spain', value: 'geoip:es' },
  { label: '🇮🇩 Indonesia', value: 'geoip:id' },
  { label: '🇺🇦 Ukraine', value: 'geoip:ua' },
  { label: '🇹🇷 Türkiye', value: 'geoip:tr' },
  { label: '🇧🇷 Brazil', value: 'geoip:br' },
];
const directDomainsOptions = [
  { label: 'Private DNS', value: 'geosite:private' },
  { label: '🇮🇷 Iran', value: 'geosite:category-ir' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇷🇺 Russia', value: 'geosite:category-ru' },
  { label: 'Apple', value: 'geosite:apple' },
  { label: 'Meta', value: 'geosite:meta' },
  { label: 'Google', value: 'geosite:google' },
];
const blockDomainsOptions = [
  { label: 'Ads All', value: 'geosite:category-ads-all' },
  { label: 'Adult +18', value: 'geosite:category-porn' },
];

function readJson<T>(raw: string, fallback: T): T {
  try {
    if (!raw) return fallback;
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

function readRules(raw: string): SubJsonRule[] {
  const parsed = readJson<unknown>(raw, null);
  return Array.isArray(parsed) ? (parsed as SubJsonRule[]) : [];
}

function blockFirst(rules: SubJsonRule[]): SubJsonRule[] {
  return [...rules].sort(
    (a, b) => Number(a.outboundTag !== 'block') - Number(b.outboundTag !== 'block'),
  );
}

export default function SubscriptionFormatsTab({
  allSetting,
  updateSetting,
}: SubscriptionFormatsTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();

  const muxEnabled = allSetting.subJsonMux !== '';

  const muxObj = useMemo(
    () =>
      muxEnabled ? readJson<typeof DEFAULT_MUX>(allSetting.subJsonMux, DEFAULT_MUX) : DEFAULT_MUX,
    [allSetting.subJsonMux, muxEnabled],
  );

  function setMuxEnabled(v: boolean) {
    updateSetting({ subJsonMux: v ? JSON.stringify(DEFAULT_MUX) : '' });
  }

  function setMuxField<K extends keyof typeof DEFAULT_MUX>(key: K, value: (typeof DEFAULT_MUX)[K]) {
    const next = { ...muxObj, [key]: value };
    updateSetting({ subJsonMux: JSON.stringify(next) });
  }

  const ruleArray = useMemo(() => readRules(allSetting.subJsonRules), [allSetting.subJsonRules]);
  const directEnabled = ruleArray.some((r) => r.outboundTag === 'direct');
  const blockEnabled = ruleArray.some((r) => r.outboundTag === 'block');

  const ruleValues = (tag: string, key: 'ip' | 'domain') =>
    ruleArray.find((r) => r.outboundTag === tag && r[key])?.[key] ?? [];

  function writeRules(rules: SubJsonRule[]) {
    updateSetting({ subJsonRules: rules.length > 0 ? JSON.stringify(blockFirst(rules)) : '' });
  }

  function setTagEnabled(tag: string, defaults: SubJsonRule[], enabled: boolean) {
    const rest = ruleArray.filter((r) => r.outboundTag !== tag);
    writeRules(enabled ? [...rest, ...defaults] : rest);
  }

  function setRuleValues(tag: string, key: 'ip' | 'domain', template: SubJsonRule, value: string[]) {
    let rules = [...ruleArray];
    if (value.length === 0) {
      rules = rules.filter((r) => !(r.outboundTag === tag && r[key]));
    } else {
      let index = rules.findIndex((r) => r.outboundTag === tag && r[key]);
      if (index === -1) {
        rules.push({ ...template });
        index = rules.length - 1;
      }
      rules[index] = { ...rules[index], [key]: [...value] };
    }
    writeRules(rules);
  }

  return (
    <Tabs
      defaultActiveKey="1"
      items={[
        {
          key: '1',
          label: catTabLabel(<SettingOutlined />, t('pages.settings.panelSettings'), isMobile),
          children: (
            <div className="subscription-format-sections">
              {allSetting.subJsonEnable && (
                <Card
                  size="small"
                  className="subscription-format-card"
                  title={
                    <span className="subscription-format-card-title">
                      <FileTextOutlined />
                      {t('pages.settings.subJsonEnableTitle')}
                    </span>
                  }
                >
                  <SettingListItem
                    paddings="small"
                    title={<>JSON {t('pages.settings.subPath')}</>}
                    description={t('pages.settings.subPathDesc')}
                  >
                    <Input
                      value={allSetting.subJsonPath}
                      placeholder="/json/"
                      onChange={(e) => updateSetting({ subJsonPath: sanitizePath(e.target.value) })}
                      onBlur={() =>
                        updateSetting({ subJsonPath: normalizePath(allSetting.subJsonPath) })
                      }
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={<>JSON {t('pages.settings.subURI')}</>}
                    description={t('pages.settings.subURIDesc')}
                  >
                    <Input
                      value={allSetting.subJsonURI}
                      placeholder="(http|https)://domain[:port]/path/"
                      onChange={(e) => updateSetting({ subJsonURI: e.target.value })}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subJsonAlwaysArray')}
                    description={t('pages.settings.subJsonAlwaysArrayDesc')}
                  >
                    <Switch
                      checked={allSetting.subJsonAlwaysArray}
                      onChange={(value) => updateSetting({ subJsonAlwaysArray: value })}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subJsonAutoDetect')}
                    description={t('pages.settings.subJsonAutoDetectDesc')}
                  >
                    <Switch
                      checked={allSetting.subJsonAutoDetect}
                      onChange={(v) => updateSetting({ subJsonAutoDetect: v })}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subJsonUserAgentRegex')}
                    description={t('pages.settings.subJsonUserAgentRegexDesc')}
                  >
                    <GoRegexInput
                      value={allSetting.subJsonUserAgentRegex}
                      placeholder="(?i)^myclient([ /]|$)"
                      onChange={(value) => updateSetting({ subJsonUserAgentRegex: value })}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subJsonRoutingRules')}
                    badge={remoteSourceBadge(allSetting.subJsonRoutingRules)}
                    description={t('pages.settings.subJsonRoutingRulesDesc')}
                  >
                    <Input.TextArea
                      value={allSetting.subJsonRoutingRules}
                      placeholder="happ://routing/onadd/... , routing JSON, or https://.../DEFAULT.JSON"
                      onChange={(e) => updateSetting({ subJsonRoutingRules: e.target.value })}
                      autoSize={{ minRows: 2, maxRows: 6 }}
                    />
                  </SettingListItem>
                </Card>
              )}
              {allSetting.subClashEnable && (
                <Card
                  size="small"
                  className="subscription-format-card"
                  title={
                    <span className="subscription-format-card-title">
                      <NodeIndexOutlined />
                      {t('pages.settings.subClashEnableTitle')}
                    </span>
                  }
                >
                  <SettingListItem
                    paddings="small"
                    title={<>Clash {t('pages.settings.subPath')}</>}
                    description={t('pages.settings.subPathDesc')}
                  >
                    <Input
                      value={allSetting.subClashPath}
                      placeholder="/clash/"
                      onChange={(e) =>
                        updateSetting({ subClashPath: sanitizePath(e.target.value) })
                      }
                      onBlur={() =>
                        updateSetting({ subClashPath: normalizePath(allSetting.subClashPath) })
                      }
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={<>Clash {t('pages.settings.subURI')}</>}
                    description={t('pages.settings.subURIDesc')}
                  >
                    <Input
                      value={allSetting.subClashURI}
                      placeholder="(http|https)://domain[:port]/path/"
                      onChange={(e) => updateSetting({ subClashURI: e.target.value })}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subClashAutoDetect')}
                    description={t('pages.settings.subClashAutoDetectDesc')}
                  >
                    <Switch
                      checked={allSetting.subClashAutoDetect}
                      onChange={(v) => updateSetting({ subClashAutoDetect: v })}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subClashUserAgentRegex')}
                    description={t('pages.settings.subClashUserAgentRegexDesc')}
                  >
                    <GoRegexInput
                      value={allSetting.subClashUserAgentRegex}
                      placeholder="(?i)(clash|mihomo)"
                      onChange={(value) => updateSetting({ subClashUserAgentRegex: value })}
                    />
                  </SettingListItem>
                </Card>
              )}
            </div>
          ),
        },
        {
          key: '2',
          label: catTabLabel(
            <RocketOutlined />,
            t('pages.settings.subFormats.finalMask'),
            isMobile,
          ),
          children: (
            <>
              <SettingListItem
                paddings="small"
                title={t('pages.settings.subFormats.finalMask')}
                description={t('pages.settings.subFormats.finalMaskDesc')}
              />
              <SubJsonFinalMaskForm
                value={allSetting.subJsonFinalMask}
                onChange={(v) => updateSetting({ subJsonFinalMask: v })}
              />
            </>
          ),
        },
        {
          key: '3',
          label: catTabLabel(<PartitionOutlined />, t('pages.settings.mux'), isMobile),
          children: (
            <>
              <SettingListItem
                paddings="small"
                title={t('pages.settings.mux')}
                description={t('pages.settings.muxDesc')}
              >
                <Switch checked={muxEnabled} onChange={setMuxEnabled} />
              </SettingListItem>
              {muxEnabled && (
                <div className="format-settings">
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subFormats.concurrency')}
                  >
                    <InputNumber
                      value={muxObj.concurrency}
                      min={-1}
                      max={1024}
                      style={{ width: '100%' }}
                      onChange={onNumber((v) => setMuxField('concurrency', v))}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subFormats.xudpConcurrency')}
                  >
                    <InputNumber
                      value={muxObj.xudpConcurrency}
                      min={-1}
                      max={1024}
                      style={{ width: '100%' }}
                      onChange={onNumber((v) => setMuxField('xudpConcurrency', v))}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.subFormats.xudpUdp443')}
                  >
                    <Select
                      value={muxObj.xudpProxyUDP443}
                      style={{ width: '100%' }}
                      onChange={(v) => setMuxField('xudpProxyUDP443', v)}
                      options={['reject', 'allow', 'skip'].map((p) => ({ value: p, label: p }))}
                    />
                  </SettingListItem>
                </div>
              )}
            </>
          ),
        },
        {
          key: '4',
          label: catTabLabel(<SendOutlined />, t('pages.settings.direct'), isMobile),
          children: (
            <>
              <SettingListItem
                paddings="small"
                title={t('pages.settings.direct')}
                description={t('pages.settings.directDesc')}
              >
                <Switch
                  checked={directEnabled}
                  onChange={(v) => setTagEnabled('direct', DEFAULT_DIRECT_RULES, v)}
                />
              </SettingListItem>
              {directEnabled && (
                <div className="format-settings">
                  <SettingListItem paddings="small" title={<>{t('pages.settings.direct')} IPs</>}>
                    <Select
                      mode="tags"
                      value={ruleValues('direct', 'ip')}
                      style={{ width: '100%' }}
                      onChange={(v) => setRuleValues('direct', 'ip', DEFAULT_DIRECT_RULES[1], v)}
                      options={directIPsOptions}
                    />
                  </SettingListItem>
                  <SettingListItem
                    paddings="small"
                    title={
                      <>
                        {t('pages.settings.direct')} {t('domainName')}
                      </>
                    }
                  >
                    <Select
                      mode="tags"
                      value={ruleValues('direct', 'domain')}
                      style={{ width: '100%' }}
                      onChange={(v) =>
                        setRuleValues('direct', 'domain', DEFAULT_DIRECT_RULES[0], v)
                      }
                      options={directDomainsOptions}
                    />
                  </SettingListItem>
                </div>
              )}
            </>
          ),
        },
        {
          key: '5',
          label: catTabLabel(<StopOutlined />, t('pages.settings.block'), isMobile),
          children: (
            <>
              <SettingListItem
                paddings="small"
                title={t('pages.settings.block')}
                description={t('pages.settings.blockDesc')}
              >
                <Switch
                  checked={blockEnabled}
                  onChange={(v) => setTagEnabled('block', DEFAULT_BLOCK_RULES, v)}
                />
              </SettingListItem>
              {blockEnabled && (
                <div className="format-settings">
                  <SettingListItem paddings="small" title={t('pages.xray.blockdomains')}>
                    <Select
                      mode="tags"
                      value={ruleValues('block', 'domain')}
                      style={{ width: '100%' }}
                      onChange={(v) =>
                        setRuleValues('block', 'domain', DEFAULT_BLOCK_RULES[0], v)
                      }
                      options={blockDomainsOptions}
                    />
                  </SettingListItem>
                  <SettingListItem paddings="small" title={t('pages.xray.blockips')}>
                    <Select
                      mode="tags"
                      value={ruleValues('block', 'ip')}
                      style={{ width: '100%' }}
                      onChange={(v) => setRuleValues('block', 'ip', BLOCK_IP_RULE, v)}
                      options={directIPsOptions}
                    />
                  </SettingListItem>
                </div>
              )}
            </>
          ),
        },
      ]}
    />
  );
}
