import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Select, Space, Switch, Tabs } from 'antd';
import { BellOutlined, SettingOutlined } from '@ant-design/icons';
import { LanguageManager } from '@/utils';
import { HttpUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';

interface TelegramTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

// The notification schedule is fed straight to robfig/cron's AddJob (see
// web.go startTask), which accepts @every <duration>, the @hourly/@daily/...
// macros, and full crontab expressions. This builder covers the common cases
// with dropdowns so users don't have to memorise the syntax, while "Custom"
// preserves the raw crontab escape hatch.
type Unit = 's' | 'm' | 'h';
type Macro = '@hourly' | '@daily' | '@weekly' | '@monthly';
type Mode = 'every' | Macro | 'custom';
const MACROS: Macro[] = ['@hourly', '@daily', '@weekly', '@monthly'];
const EVERY_RE = /^@every\s+(\d+)\s*([smh])$/i;

interface RunTime {
  mode: Mode;
  num: number;
  unit: Unit;
  custom: string;
}

function parseRunTime(raw: string): RunTime {
  const v = (raw ?? '').trim();
  const m = v.match(EVERY_RE);
  if (m) {
    return { mode: 'every', num: Math.max(1, Number(m[1]) || 1), unit: m[2].toLowerCase() as Unit, custom: '' };
  }
  if ((MACROS as string[]).includes(v)) {
    return { mode: v as Macro, num: 1, unit: 'h', custom: '' };
  }
  return { mode: 'custom', num: 1, unit: 'h', custom: v };
}

function composeRunTime(s: RunTime): string {
  if (s.mode === 'every') return `@every ${Math.max(1, s.num || 1)}${s.unit}`;
  if (s.mode === 'custom') return s.custom;
  return s.mode;
}

// The panel's cron runs with seconds enabled (cron.WithSeconds() in web.go), so
// crontab expressions are 6-field: "second minute hour day month weekday". When
// the user drops into Custom we seed the box with the crontab equivalent of the
// current selection rather than a bare @macro, so they get a real expression to
// edit (and one that the 6-field parser accepts).
function toCrontab(s: RunTime): string {
  switch (s.mode) {
    case '@hourly': return '0 0 * * * *';
    case '@daily': return '0 0 0 * * *';
    case '@weekly': return '0 0 0 * * 0';
    case '@monthly': return '0 0 0 1 * *';
    case 'every': {
      const n = Math.max(1, s.num || 1);
      if (s.unit === 's') return `*/${n} * * * * *`;
      if (s.unit === 'm') return `0 */${n} * * * *`;
      return `0 0 */${n} * * *`;
    }
    default: return s.custom;
  }
}

function NotifyTimeField({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const { t } = useTranslation();
  // Init once: the Settings tabs only mount after settings are fetched, so the
  // incoming value is already the persisted one.
  const [state, setState] = useState<RunTime>(() => parseRunTime(value));

  function update(patch: Partial<RunTime>) {
    const next = { ...state, ...patch };
    setState(next);
    onChange(composeRunTime(next));
  }

  function onModeChange(mode: Mode) {
    // Seed Custom with the crontab equivalent of the current selection so the
    // box starts from a real expression (e.g. "0 0 0 * * *", not "@daily").
    if (mode === 'custom' && !state.custom.trim()) {
      update({ mode, custom: toCrontab(state) });
    } else {
      update({ mode });
    }
  }

  const modeOptions = [
    { value: 'every', label: t('pages.settings.notifyTime.every') },
    { value: '@hourly', label: t('pages.settings.notifyTime.hourly') },
    { value: '@daily', label: t('pages.settings.notifyTime.daily') },
    { value: '@weekly', label: t('pages.settings.notifyTime.weekly') },
    { value: '@monthly', label: t('pages.settings.notifyTime.monthly') },
    { value: 'custom', label: t('pages.settings.notifyTime.custom') },
  ];
  const unitOptions = [
    { value: 's', label: t('pages.settings.notifyTime.seconds') },
    { value: 'm', label: t('pages.settings.notifyTime.minutes') },
    { value: 'h', label: t('pages.settings.notifyTime.hours') },
  ];

  return (
    <Space direction="vertical" size="small" style={{ width: '100%' }}>
      <Select<Mode>
        style={{ width: '100%' }}
        value={state.mode}
        options={modeOptions}
        onChange={onModeChange}
      />
      {state.mode === 'every' && (
        <Space.Compact style={{ width: '100%' }}>
          <InputNumber
            min={1}
            style={{ width: '50%' }}
            value={state.num}
            onChange={(v) => update({ num: Math.max(1, Number(v) || 1) })}
          />
          <Select<Unit>
            style={{ width: '50%' }}
            value={state.unit}
            options={unitOptions}
            onChange={(unit) => update({ unit })}
          />
        </Space.Compact>
      )}
      {state.mode === 'custom' && (
        <Input
          value={state.custom}
          placeholder="0 30 8 * * *"
          onChange={(e) => update({ custom: e.target.value })}
        />
      )}
    </Space>
  );
}

export default function TelegramTab({ allSetting, updateSetting }: TelegramTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [outboundTagList, setOutboundTagList] = useState<string[]>([]);
  const [balancerTagList, setBalancerTagList] = useState<string[]>([]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const msg = await HttpUtil.post('/panel/api/xray/', undefined, { silent: true }) as { success?: boolean; obj?: string };
      if (cancelled || !msg?.success || typeof msg.obj !== 'string') return;
      try {
        const payload = JSON.parse(msg.obj) as Record<string, unknown>;
        const template = (payload.xraySetting || {}) as Record<string, unknown>;
        const tags = new Set<string>();
        const outbounds = Array.isArray(template.outbounds) ? template.outbounds : [];
        for (const o of outbounds) {
          if (!o || typeof o !== 'object') continue;
          const rec = o as Record<string, unknown>;
          if (rec.protocol === 'blackhole') continue;
          const tag = rec.tag;
          if (typeof tag === 'string' && tag) tags.add(tag);
        }
        const subTags = Array.isArray(payload.subscriptionOutboundTags) ? payload.subscriptionOutboundTags : [];
        for (const tag of subTags) {
          if (typeof tag === 'string' && tag) tags.add(tag);
        }
        const balancerTags: string[] = [];
        const routing = (template.routing || {}) as Record<string, unknown>;
        const balancers = Array.isArray(routing.balancers) ? routing.balancers : [];
        for (const b of balancers) {
          if (!b || typeof b !== 'object') continue;
          const tag = (b as Record<string, unknown>).tag;
          if (typeof tag === 'string' && tag && !tags.has(tag)) balancerTags.push(tag);
        }
        setOutboundTagList([...tags]);
        setBalancerTagList(balancerTags);
      } catch {
        setOutboundTagList([]);
        setBalancerTagList([]);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const outboundOptions = useMemo<
    ({ label: string; value: string } | { label: string; options: { label: string; value: string }[] })[]
  >(() => {
    const outOpts = outboundTagList.map((tag) => ({ label: tag, value: tag }));
    if (balancerTagList.length === 0) return outOpts;
    return [
      { label: t('pages.xray.Outbounds'), options: outOpts },
      { label: t('pages.xray.Balancers'), options: balancerTagList.map((tag) => ({ label: tag, value: tag })) },
    ];
  }, [outboundTagList, balancerTagList, t]);

  const langOptions = useMemo(
    () => LanguageManager.supportedLanguages.map((l: { value: string; name: string; icon: string }) => ({
      value: l.value,
      label: (
        <>
          <span role="img" aria-label={l.name}>{l.icon}</span>
          &nbsp;&nbsp;<span>{l.name}</span>
        </>
      ),
    })),
    [],
  );

  return (
    <Tabs defaultActiveKey="1" items={[
      {
        key: '1',
        label: catTabLabel(<SettingOutlined />, t('pages.settings.panelSettings'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.telegramBotEnable')} description={t('pages.settings.telegramBotEnableDesc')}>
              <Switch checked={allSetting.tgBotEnable} onChange={(v) => updateSetting({ tgBotEnable: v })} />
            </SettingListItem>

            <SettingListItem
              paddings="small"
              title={t('pages.settings.telegramToken')}
              description={allSetting.hasTgBotToken ? 'Configured; leave blank to keep current token.' : t('pages.settings.telegramTokenDesc')}
            >
              <Input.Password
                value={allSetting.tgBotToken}
                placeholder={allSetting.hasTgBotToken ? 'Configured - enter a new token to replace' : ''}
                onChange={(e) => updateSetting({ tgBotToken: e.target.value })}
              />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.telegramChatId')} description={t('pages.settings.telegramChatIdDesc')}>
              <Input value={allSetting.tgBotChatId} onChange={(e) => updateSetting({ tgBotChatId: e.target.value })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.telegramBotLanguage')}>
              <Select
                value={allSetting.tgLang}
                onChange={(v) => updateSetting({ tgLang: v })}
                style={{ width: '100%' }}
                options={langOptions}
              />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.telegramAPIServer')} description={t('pages.settings.telegramAPIServerDesc')}>
              <Input value={allSetting.tgBotAPIServer} placeholder="https://api.example.com"
                onChange={(e) => updateSetting({ tgBotAPIServer: e.target.value })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.tgBotOutbound')} description={t('pages.settings.tgBotOutboundDesc')}>
              <Select
                style={{ width: '100%' }}
                allowClear
                showSearch
                value={allSetting.tgBotOutbound || undefined}
                placeholder={t('pages.settings.tgBotOutboundPh')}
                options={outboundOptions}
                onChange={(v) => updateSetting({ tgBotOutbound: (v as string | undefined) || '' })}
              />
            </SettingListItem>
          </>
        ),
      },
      {
        key: '2',
        label: catTabLabel(<BellOutlined />, t('pages.settings.notifications'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.telegramNotifyTime')} description={t('pages.settings.telegramNotifyTimeDesc')}>
              <NotifyTimeField value={allSetting.tgRunTime} onChange={(v) => updateSetting({ tgRunTime: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.tgNotifyBackup')} description={t('pages.settings.tgNotifyBackupDesc')}>
              <Switch checked={allSetting.tgBotBackup} onChange={(v) => updateSetting({ tgBotBackup: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.tgNotifyLogin')} description={t('pages.settings.tgNotifyLoginDesc')}>
              <Switch checked={allSetting.tgBotLoginNotify} onChange={(v) => updateSetting({ tgBotLoginNotify: v })} />
            </SettingListItem>
            <SettingListItem paddings="small" title={t('pages.settings.tgNotifyCpu')} description={t('pages.settings.tgNotifyCpuDesc')}>
              <InputNumber value={allSetting.tgCpu} min={0} max={100} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ tgCpu: Number(v) || 0 })} />
            </SettingListItem>
          </>
        ),
      },
    ]} />
  );
}
