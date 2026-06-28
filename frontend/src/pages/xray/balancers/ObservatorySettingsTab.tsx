import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Empty, Input, InputNumber, Segmented, Select, Space, Switch, Tag } from 'antd';

import { SettingListItem } from '@/components/ui';
import {
  BurstObservatorySchema,
  ObservatoryHttpMethodSchema,
  ObservatorySchema,
  type BurstObservatoryObject,
  type ObservatoryHttpMethod,
  type ObservatoryObject,
  type PingConfigObject,
} from '@/schemas/observatory';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

interface ObservatorySettingsTabProps {
  templateSettings: XraySettingsValue | null;
  mutate: (mutator: (next: XraySettingsValue) => void) => void;
  isMobile: boolean;
}

const OBSERVATORY_DEFAULTS = ObservatorySchema.parse({});
const BURST_DEFAULTS = BurstObservatorySchema.parse({});

function asObject(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {};
}

function SelectorTags({ tags }: { tags: string[] }) {
  if (!tags || tags.length === 0) return <Tag>—</Tag>;
  return (
    <>
      {tags.map((sel) => (
        <Tag key={sel} className="info-large-tag" style={{ margin: 0, marginRight: 4, marginBottom: 4 }}>
          {sel}
        </Tag>
      ))}
    </>
  );
}

export default function ObservatorySettingsTab({
  templateSettings,
  mutate,
  isMobile,
}: ObservatorySettingsTabProps) {
  const { t } = useTranslation();

  const observatory = useMemo<ObservatoryObject | null>(() => {
    const raw = templateSettings?.observatory;
    if (raw == null) return null;
    return { ...OBSERVATORY_DEFAULTS, ...asObject(raw) } as ObservatoryObject;
  }, [templateSettings?.observatory]);

  const burst = useMemo<BurstObservatoryObject | null>(() => {
    const raw = templateSettings?.burstObservatory;
    if (raw == null) return null;
    const merged = { ...BURST_DEFAULTS, ...asObject(raw) } as BurstObservatoryObject;
    merged.pingConfig = { ...BURST_DEFAULTS.pingConfig, ...asObject(merged.pingConfig) } as PingConfigObject;
    return merged;
  }, [templateSettings?.burstObservatory]);

  const hasObservatory = observatory != null;
  const hasBurst = burst != null;

  const [view, setView] = useState<'observatory' | 'burstObservatory'>('observatory');
  const effectiveView = !hasObservatory && hasBurst
    ? 'burstObservatory'
    : !hasBurst && hasObservatory
      ? 'observatory'
      : view;

  function patchObservatory(patch: Partial<ObservatoryObject>) {
    mutate((tt) => {
      tt.observatory = { ...OBSERVATORY_DEFAULTS, ...asObject(tt.observatory), ...patch };
    });
  }

  function patchPingConfig(patch: Partial<PingConfigObject>) {
    mutate((tt) => {
      const current = asObject(tt.burstObservatory);
      const currentPing = asObject(current.pingConfig);
      tt.burstObservatory = {
        ...BURST_DEFAULTS,
        ...current,
        pingConfig: { ...BURST_DEFAULTS.pingConfig, ...currentPing, ...patch },
      };
    });
  }

  if (!hasObservatory && !hasBurst) {
    return <Empty description={t('pages.xray.observatory.emptyHint')} />;
  }

  const observatorySection = observatory && (
    <>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.subjectSelector')}
        description={t('pages.xray.observatory.subjectSelectorDesc')}
      >
        <SelectorTags tags={observatory.subjectSelector} />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.probeURL')}
        description={t('pages.xray.observatory.probeURLDesc')}
      >
        <Input
          value={observatory.probeURL}
          onChange={(e) => patchObservatory({ probeURL: e.target.value })}
          placeholder="https://www.google.com/generate_204"
        />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.probeInterval')}
        description={t('pages.xray.observatory.probeIntervalDesc')}
      >
        <Input
          value={observatory.probeInterval}
          onChange={(e) => patchObservatory({ probeInterval: e.target.value })}
          placeholder="1m"
        />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.enableConcurrency')}
        description={t('pages.xray.observatory.enableConcurrencyDesc')}
      >
        <Switch
          checked={observatory.enableConcurrency}
          onChange={(v) => patchObservatory({ enableConcurrency: v })}
        />
      </SettingListItem>
    </>
  );

  const burstSection = burst && (
    <>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.subjectSelector')}
        description={t('pages.xray.observatory.subjectSelectorDesc')}
      >
        <SelectorTags tags={burst.subjectSelector} />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.destination')}
        description={t('pages.xray.observatory.destinationDesc')}
      >
        <Input
          value={burst.pingConfig.destination}
          onChange={(e) => patchPingConfig({ destination: e.target.value })}
          placeholder="https://www.google.com/generate_204"
        />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.connectivity')}
        description={t('pages.xray.observatory.connectivityDesc')}
      >
        <Input
          value={burst.pingConfig.connectivity}
          allowClear
          onChange={(e) => patchPingConfig({ connectivity: e.target.value })}
          placeholder="http://connectivitycheck.platform.hicloud.com/generate_204"
        />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.interval')}
        description={t('pages.xray.observatory.intervalDesc')}
      >
        <Input
          value={burst.pingConfig.interval}
          onChange={(e) => patchPingConfig({ interval: e.target.value })}
          placeholder="1m"
        />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.timeout')}
        description={t('pages.xray.observatory.timeoutDesc')}
      >
        <Input
          value={burst.pingConfig.timeout}
          onChange={(e) => patchPingConfig({ timeout: e.target.value })}
          placeholder="5s"
        />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.sampling')}
        description={t('pages.xray.observatory.samplingDesc')}
      >
        <InputNumber
          min={1}
          value={burst.pingConfig.sampling}
          onChange={(v) => patchPingConfig({ sampling: typeof v === 'number' ? v : burst.pingConfig.sampling })}
          style={{ width: '100%' }}
        />
      </SettingListItem>
      <SettingListItem
        paddings="small"
        title={t('pages.xray.observatory.httpMethod')}
        description={t('pages.xray.observatory.httpMethodDesc')}
      >
        <Select<ObservatoryHttpMethod>
          value={burst.pingConfig.httpMethod}
          onChange={(v) => patchPingConfig({ httpMethod: v })}
          options={ObservatoryHttpMethodSchema.options.map((m) => ({ value: m, label: m }))}
          style={{ width: '100%' }}
        />
      </SettingListItem>
    </>
  );

  return (
    <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
      <Alert type="info" showIcon message={t('pages.xray.observatory.autoManaged')} />
      {hasObservatory && hasBurst && (
        <Segmented
          block={isMobile}
          value={effectiveView}
          onChange={(v) => setView(v as 'observatory' | 'burstObservatory')}
          options={[
            { label: t('pages.xray.observatory.title'), value: 'observatory' },
            { label: t('pages.xray.observatory.burstTitle'), value: 'burstObservatory' },
          ]}
        />
      )}
      <div>{effectiveView === 'observatory' ? observatorySection : burstSection}</div>
    </Space>
  );
}
