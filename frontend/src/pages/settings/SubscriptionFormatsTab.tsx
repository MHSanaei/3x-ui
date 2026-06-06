import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Input,
  InputNumber,
  Select,
  Switch,
  Tabs,
} from 'antd';
import {
  PartitionOutlined,
  RocketOutlined,
  SendOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import { sanitizePath, normalizePath } from './uriPath';
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
const DEFAULT_RULES: { type: string; outboundTag: string; domain?: string[]; ip?: string[] }[] = [
  { type: 'field', outboundTag: 'direct', domain: ['geosite:category-ir'] },
  { type: 'field', outboundTag: 'direct', ip: ['geoip:private', 'geoip:ir'] },
];

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

function readJson<T>(raw: string, fallback: T): T {
  try {
    if (!raw) return fallback;
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

export default function SubscriptionFormatsTab({ allSetting, updateSetting }: SubscriptionFormatsTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();

  const muxEnabled = allSetting.subJsonMux !== '';
  const directEnabled = allSetting.subJsonRules !== '';

  const muxObj = useMemo(
    () => (muxEnabled ? readJson<typeof DEFAULT_MUX>(allSetting.subJsonMux, DEFAULT_MUX) : DEFAULT_MUX),
    [allSetting.subJsonMux, muxEnabled],
  );

  function setMuxEnabled(v: boolean) {
    updateSetting({ subJsonMux: v ? JSON.stringify(DEFAULT_MUX) : '' });
  }

  function setMuxField<K extends keyof typeof DEFAULT_MUX>(key: K, value: typeof DEFAULT_MUX[K]) {
    const next = { ...muxObj, [key]: value };
    updateSetting({ subJsonMux: JSON.stringify(next) });
  }

  const ruleArray = useMemo(() => {
    if (!directEnabled) return null;
    return readJson<typeof DEFAULT_RULES | null>(allSetting.subJsonRules, null);
  }, [allSetting.subJsonRules, directEnabled]);

  const directIPs = useMemo(() => {
    if (!ruleArray) return [];
    const ipRule = ruleArray.find((r) => r.ip);
    return ipRule?.ip ?? [];
  }, [ruleArray]);

  const directDomains = useMemo(() => {
    if (!ruleArray) return [];
    const dRule = ruleArray.find((r) => r.domain);
    return dRule?.domain ?? [];
  }, [ruleArray]);

  function setDirectEnabled(v: boolean) {
    updateSetting({ subJsonRules: v ? JSON.stringify(DEFAULT_RULES) : '' });
  }

  function setDirectIPs(value: string[]) {
    if (!ruleArray) return;
    let rules = [...ruleArray];
    if (value.length === 0) {
      rules = rules.filter((r) => !r.ip);
    } else {
      let idx = rules.findIndex((r) => r.ip);
      if (idx === -1) {
        rules.push({ ...DEFAULT_RULES[1] });
        idx = rules.length - 1;
      }
      rules[idx] = { ...rules[idx], ip: [...value] };
    }
    updateSetting({ subJsonRules: JSON.stringify(rules) });
  }

  function setDirectDomains(value: string[]) {
    if (!ruleArray) return;
    let rules = [...ruleArray];
    if (value.length === 0) {
      rules = rules.filter((r) => !r.domain);
    } else {
      let idx = rules.findIndex((r) => r.domain);
      if (idx === -1) {
        rules.push({ ...DEFAULT_RULES[0] });
        idx = rules.length - 1;
      }
      rules[idx] = { ...rules[idx], domain: [...value] };
    }
    updateSetting({ subJsonRules: JSON.stringify(rules) });
  }

  return (
    <Tabs defaultActiveKey="1" items={[
      {
        key: '1',
        label: catTabLabel(<SettingOutlined />, t('pages.settings.panelSettings'), isMobile),
        children: (
          <>
            {allSetting.subJsonEnable && (
              <>
                <SettingListItem paddings="small" title={<>JSON {t('pages.settings.subPath')}</>} description={t('pages.settings.subPathDesc')}>
                  <Input
                    value={allSetting.subJsonPath}
                    placeholder="/json/"
                    onChange={(e) => updateSetting({ subJsonPath: sanitizePath(e.target.value) })}
                    onBlur={() => updateSetting({ subJsonPath: normalizePath(allSetting.subJsonPath) })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={<>JSON {t('pages.settings.subURI')}</>} description={t('pages.settings.subURIDesc')}>
                  <Input
                    value={allSetting.subJsonURI}
                    placeholder="(http|https)://domain[:port]/path/"
                    onChange={(e) => updateSetting({ subJsonURI: e.target.value })}
                  />
                </SettingListItem>
              </>
            )}
            {allSetting.subClashEnable && (
              <>
                <SettingListItem paddings="small" title={<>Clash {t('pages.settings.subPath')}</>} description={t('pages.settings.subPathDesc')}>
                  <Input
                    value={allSetting.subClashPath}
                    placeholder="/clash/"
                    onChange={(e) => updateSetting({ subClashPath: sanitizePath(e.target.value) })}
                    onBlur={() => updateSetting({ subClashPath: normalizePath(allSetting.subClashPath) })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={<>Clash {t('pages.settings.subURI')}</>} description={t('pages.settings.subURIDesc')}>
                  <Input
                    value={allSetting.subClashURI}
                    placeholder="(http|https)://domain[:port]/path/"
                    onChange={(e) => updateSetting({ subClashURI: e.target.value })}
                  />
                </SettingListItem>
              </>
            )}
          </>
        ),
      },
      {
        key: '2',
        label: catTabLabel(<RocketOutlined />, t('pages.settings.subFormats.finalMask'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subFormats.finalMask')} description={t('pages.settings.subFormats.finalMaskDesc')} />
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
            <SettingListItem paddings="small" title={t('pages.settings.mux')} description={t('pages.settings.muxDesc')}>
              <Switch checked={muxEnabled} onChange={setMuxEnabled} />
            </SettingListItem>
            {muxEnabled && (
              <div className="format-settings">
                <SettingListItem paddings="small" title={t('pages.settings.subFormats.concurrency')}>
                  <InputNumber value={muxObj.concurrency} min={-1} max={1024} style={{ width: '100%' }}
                    onChange={(v) => setMuxField('concurrency', Number(v) || 0)} />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subFormats.xudpConcurrency')}>
                  <InputNumber value={muxObj.xudpConcurrency} min={-1} max={1024} style={{ width: '100%' }}
                    onChange={(v) => setMuxField('xudpConcurrency', Number(v) || 0)} />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subFormats.xudpUdp443')}>
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
            <SettingListItem paddings="small" title={t('pages.settings.direct')} description={t('pages.settings.directDesc')}>
              <Switch checked={directEnabled} onChange={setDirectEnabled} />
            </SettingListItem>
            {directEnabled && (
              <div className="format-settings">
                <SettingListItem paddings="small" title={<>{t('pages.settings.direct')} IPs</>}>
                  <Select
                    mode="tags"
                    value={directIPs}
                    style={{ width: '100%' }}
                    onChange={setDirectIPs}
                    options={directIPsOptions}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={<>{t('pages.settings.direct')} {t('domainName')}</>}>
                  <Select
                    mode="tags"
                    value={directDomains}
                    style={{ width: '100%' }}
                    onChange={setDirectDomains}
                    options={directDomainsOptions}
                  />
                </SettingListItem>
              </div>
            )}
          </>
        ),
      },
    ]} />
  );
}
