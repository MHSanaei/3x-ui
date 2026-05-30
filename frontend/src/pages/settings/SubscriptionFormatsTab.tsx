import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Collapse,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
} from 'antd';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { sanitizePath, normalizePath } from './uriPath';
import './SubscriptionFormatsTab.css';

interface SubscriptionFormatsTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

const DEFAULT_FRAGMENT = {
  packets: 'tlshello',
  length: '100-200',
  interval: '10-20',
  maxSplit: '300-400',
};
const DEFAULT_NOISES: { type: string; packet: string; delay: string; applyTo: string }[] = [
  { type: 'rand', packet: '10-20', delay: '10-16', applyTo: 'ip' },
];
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

  const fragment = allSetting.subJsonFragment !== '';
  const noisesEnabled = allSetting.subJsonNoises !== '';
  const muxEnabled = allSetting.subJsonMux !== '';
  const directEnabled = allSetting.subJsonRules !== '';

  const fragmentObj = useMemo(
    () => (fragment ? readJson<typeof DEFAULT_FRAGMENT>(allSetting.subJsonFragment, DEFAULT_FRAGMENT) : DEFAULT_FRAGMENT),
    [allSetting.subJsonFragment, fragment],
  );

  function setFragmentEnabled(v: boolean) {
    updateSetting({ subJsonFragment: v ? JSON.stringify(DEFAULT_FRAGMENT) : '' });
  }

  function setFragmentField<K extends keyof typeof DEFAULT_FRAGMENT>(key: K, value: string) {
    if (value === '') return;
    const next = { ...fragmentObj, [key]: value };
    updateSetting({ subJsonFragment: JSON.stringify(next) });
  }

  const noisesArray = useMemo(
    () => (noisesEnabled ? readJson<typeof DEFAULT_NOISES>(allSetting.subJsonNoises, DEFAULT_NOISES) : []),
    [allSetting.subJsonNoises, noisesEnabled],
  );

  function setNoisesEnabled(v: boolean) {
    updateSetting({ subJsonNoises: v ? JSON.stringify(DEFAULT_NOISES) : '' });
  }

  function setNoisesArray(next: typeof DEFAULT_NOISES) {
    if (noisesEnabled) updateSetting({ subJsonNoises: JSON.stringify(next) });
  }

  function addNoise() {
    setNoisesArray([...noisesArray, { ...DEFAULT_NOISES[0] }]);
  }

  function removeNoise(index: number) {
    const next = [...noisesArray];
    next.splice(index, 1);
    setNoisesArray(next);
  }

  function updateNoiseField(index: number, field: keyof typeof DEFAULT_NOISES[number], value: string) {
    const next = [...noisesArray];
    next[index] = { ...next[index], [field]: value };
    setNoisesArray(next);
  }

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
    <Collapse defaultActiveKey="1" items={[
      {
        key: '1',
        label: t('pages.settings.panelSettings'),
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
        label: t('pages.settings.fragment'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.fragment')} description={t('pages.settings.fragmentDesc')}>
              <Switch checked={fragment} onChange={setFragmentEnabled} />
            </SettingListItem>
            {fragment && (
              <div className="nested-block">
                <Collapse items={[
                  {
                    key: 'sett',
                    label: t('pages.settings.fragmentSett'),
                    children: (
                      <>
                        <SettingListItem paddings="small" title={t('pages.settings.subFormats.packets')}>
                          <Input value={fragmentObj.packets} placeholder="1-1 | 1-3 | tlshello | …"
                            onChange={(e) => setFragmentField('packets', e.target.value)} />
                        </SettingListItem>
                        <SettingListItem paddings="small" title={t('pages.settings.subFormats.length')}>
                          <Input value={fragmentObj.length} placeholder="100-200"
                            onChange={(e) => setFragmentField('length', e.target.value)} />
                        </SettingListItem>
                        <SettingListItem paddings="small" title={t('pages.settings.subFormats.interval')}>
                          <Input value={fragmentObj.interval} placeholder="10-20"
                            onChange={(e) => setFragmentField('interval', e.target.value)} />
                        </SettingListItem>
                        <SettingListItem paddings="small" title={t('pages.settings.subFormats.maxSplit')}>
                          <Input value={fragmentObj.maxSplit} placeholder="300-400"
                            onChange={(e) => setFragmentField('maxSplit', e.target.value)} />
                        </SettingListItem>
                      </>
                    ),
                  },
                ]} />
              </div>
            )}
          </>
        ),
      },
      {
        key: '3',
        label: t('pages.settings.subFormats.noises'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.subFormats.noises')} description={t('pages.settings.noisesDesc')}>
              <Switch checked={noisesEnabled} onChange={setNoisesEnabled} />
            </SettingListItem>
            {noisesEnabled && (
              <div className="nested-block">
                <Collapse items={noisesArray.map((noise, index) => ({
                  key: String(index),
                  label: t('pages.settings.subFormats.noiseItem', { n: index + 1 }),
                  children: (
                    <>
                      <SettingListItem paddings="small" title={t('pages.settings.subFormats.type')}>
                        <Select
                          value={noise.type}
                          style={{ width: '100%' }}
                          onChange={(v) => updateNoiseField(index, 'type', v)}
                          options={['rand', 'base64', 'str', 'hex'].map((p) => ({ value: p, label: p }))}
                        />
                      </SettingListItem>
                      <SettingListItem paddings="small" title={t('pages.settings.subFormats.packet')}>
                        <Input value={noise.packet} placeholder="5-10"
                          onChange={(e) => updateNoiseField(index, 'packet', e.target.value)} />
                      </SettingListItem>
                      <SettingListItem paddings="small" title={t('pages.settings.subFormats.delayMs')}>
                        <Input value={noise.delay} placeholder="10-20"
                          onChange={(e) => updateNoiseField(index, 'delay', e.target.value)} />
                      </SettingListItem>
                      <SettingListItem paddings="small" title={t('pages.settings.subFormats.applyTo')}>
                        <Select
                          value={noise.applyTo}
                          style={{ width: '100%' }}
                          onChange={(v) => updateNoiseField(index, 'applyTo', v)}
                          options={['ip', 'ipv4', 'ipv6'].map((p) => ({ value: p, label: p }))}
                        />
                      </SettingListItem>
                      <Space style={{ padding: '10px 20px' }}>
                        {noisesArray.length > 1 && (
                          <Button type="primary" danger onClick={() => removeNoise(index)}>
                            {t('delete')}
                          </Button>
                        )}
                      </Space>
                    </>
                  ),
                }))} />
                <Button type="primary" style={{ marginTop: 10 }} onClick={addNoise}>{t('pages.settings.subFormats.addNoise')}</Button>
              </div>
            )}
          </>
        ),
      },
      {
        key: '4',
        label: t('pages.settings.mux'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.mux')} description={t('pages.settings.muxDesc')}>
              <Switch checked={muxEnabled} onChange={setMuxEnabled} />
            </SettingListItem>
            {muxEnabled && (
              <div className="nested-block">
                <Collapse items={[
                  {
                    key: 'sett',
                    label: t('pages.settings.muxSett'),
                    children: (
                      <>
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
                      </>
                    ),
                  },
                ]} />
              </div>
            )}
          </>
        ),
      },
      {
        key: '5',
        label: t('pages.settings.direct'),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.direct')} description={t('pages.settings.directDesc')}>
              <Switch checked={directEnabled} onChange={setDirectEnabled} />
            </SettingListItem>
            {directEnabled && (
              <div className="nested-block">
                <Collapse items={[
                  {
                    key: 'rules',
                    label: t('pages.settings.direct'),
                    children: (
                      <>
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
                      </>
                    ),
                  },
                ]} />
              </div>
            )}
          </>
        ),
      },
    ]} />
  );
}
