import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, InputNumber, Select, Space, Switch, Tabs } from 'antd';
import { BellOutlined, SendOutlined, SettingOutlined } from '@ant-design/icons';
import { LanguageManager } from '@/utils';
import { HttpUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { TelegramNotifications } from '@/components/ui/notifications/TelegramNotifications';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import SecretInput from './SecretInput';

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
    <Space orientation="vertical" size="small" style={{ width: '100%' }}>
      <Select<Mode>
        style={{ width: '100%' }}
        value={state.mode}
        options={modeOptions}
        onChange={onModeChange}
        aria-label={t('pages.settings.telegramNotifyTime')}
      />
      {state.mode === 'every' && (
        <Space.Compact style={{ width: '100%' }}>
          <InputNumber
            min={1}
            style={{ width: '50%' }}
            value={state.num}
            onChange={(v) => update({ num: Math.max(1, Number(v) || 1) })}
            aria-label={t('pages.settings.notifyTime.interval')}
          />
          <Select<Unit>
            style={{ width: '50%' }}
            value={state.unit}
            options={unitOptions}
            onChange={(unit) => update({ unit })}
            aria-label={t('pages.settings.notifyTime.unit')}
          />
        </Space.Compact>
      )}
      {state.mode === 'custom' && (
        <Input
          value={state.custom}
          placeholder="0 30 8 * * *"
          onChange={(e) => update({ custom: e.target.value })}
          aria-label={t('pages.settings.notifyTime.custom')}
        />
      )}
    </Space>
  );
}

export default function TelegramTab({ allSetting, updateSetting }: TelegramTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; msg: string } | null>(null);

  async function handleTestTgBot() {
    setTestLoading(true);
    setTestResult(null);
    try {
      const res = await HttpUtil.post('/panel/api/setting/testTgBot') as { success?: boolean; msg?: string };
      setTestResult({ success: !!res.success, msg: res.msg || '' });
    } catch (e: unknown) {
      setTestResult({ success: false, msg: e instanceof Error ? e.message : t('pages.settings.requestFailed') });
    } finally {
      setTestLoading(false);
    }
  }

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
              description={allSetting.hasTgBotToken && !allSetting.clearTgBotToken ? t('pages.settings.telegramTokenConfigured') : t('pages.settings.telegramTokenDesc')}
            >
              <SecretInput
                value={allSetting.tgBotToken}
                configured={allSetting.hasTgBotToken}
                clearArmed={allSetting.clearTgBotToken}
                placeholder={t('pages.settings.telegramTokenPlaceholder')}
                onChange={(v) => updateSetting({ tgBotToken: v })}
                onClearArmedChange={(armed) => updateSetting({ clearTgBotToken: armed })}
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

            <Space orientation="vertical" size={8} style={{ width: '100%', marginTop: 16 }}>
              <Button type="primary" icon={<SendOutlined />} loading={testLoading} onClick={handleTestTgBot}>
                {t('pages.settings.testTgBot')}
              </Button>
              {testResult && (
                <Alert
                  type={testResult.success ? 'success' : 'error'}
                  title={testResult.msg}
                  showIcon
                  closable={{ onClose: () => setTestResult(null) }}
                />
              )}
            </Space>
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

            <SettingListItem paddings="small" title={t('pages.settings.tgEventBusNotify')} description={t('pages.settings.tgEventBusNotifyDesc')}>
              <TelegramNotifications allSetting={allSetting} updateSetting={updateSetting} />
            </SettingListItem>
          </>
        ),
      },
    ]} />
  );
}
