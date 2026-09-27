import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Select, Space } from 'antd';
import { onNumber } from '@/utils/onNumber';

// The notification schedule is fed straight to robfig/cron's AddJob (see
// web.go startTask), which accepts @every <duration>, the @hourly/@daily/...
// macros, and full crontab expressions. This builder covers the common cases
// with dropdowns so users don't have to memorise the syntax, while "Custom"
// preserves the raw crontab escape hatch.
export type Unit = 's' | 'm' | 'h';
export type Macro = '@hourly' | '@daily' | '@weekly' | '@monthly';
export type Mode = 'every' | Macro | 'custom';
const MACROS: Macro[] = ['@hourly', '@daily', '@weekly', '@monthly'];
const EVERY_RE = /^@every\s+(\d+)\s*([smh])$/i;

export interface RunTime {
  mode: Mode;
  num: number;
  unit: Unit;
  custom: string;
}

export function parseRunTime(raw: string): RunTime {
  const v = (raw ?? '').trim();
  const m = v.match(EVERY_RE);
  if (m) {
    return {
      mode: 'every',
      num: Math.max(1, Number(m[1]) || 1),
      unit: m[2].toLowerCase() as Unit,
      custom: '',
    };
  }
  if ((MACROS as string[]).includes(v)) {
    return { mode: v as Macro, num: 1, unit: 'h', custom: '' };
  }
  return { mode: 'custom', num: 1, unit: 'h', custom: v };
}

export function composeRunTime(s: RunTime): string {
  if (s.mode === 'every') return `@every ${Math.max(1, s.num || 1)}${s.unit}`;
  if (s.mode === 'custom') return s.custom;
  return s.mode;
}

// The panel's cron runs with seconds enabled (cron.WithSeconds() in web.go), so
// crontab expressions are 6-field: "second minute hour day month weekday". When
// the user drops into Custom we seed the box with the crontab equivalent of the
// current selection rather than a bare @macro, so they get a real expression to
// edit (and one that the 6-field parser accepts).
export function toCrontab(s: RunTime): string {
  switch (s.mode) {
    case '@hourly':
      return '0 0 * * * *';
    case '@daily':
      return '0 0 0 * * *';
    case '@weekly':
      return '0 0 0 * * 0';
    case '@monthly':
      return '0 0 0 1 * *';
    case 'every': {
      const n = Math.max(1, s.num || 1);
      if (s.unit === 's') return `*/${n} * * * * *`;
      if (s.unit === 'm') return `0 */${n} * * * *`;
      return `0 0 */${n} * * *`;
    }
    default:
      return s.custom;
  }
}

export function NotifyTimeField({
  value,
  onChange,
  ariaLabel,
}: {
  value: string;
  onChange: (v: string) => void;
  ariaLabel?: string;
}) {
  const { t } = useTranslation();
  const [state, setState] = useState<RunTime>(() => parseRunTime(value));

  function update(patch: Partial<RunTime>) {
    const next = { ...state, ...patch };
    setState(next);
    onChange(composeRunTime(next));
  }

  function onModeChange(mode: Mode) {
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
        aria-label={ariaLabel || t('pages.settings.telegramNotifyTime')}
      />
      {state.mode === 'every' && (
        <Space.Compact style={{ width: '100%' }}>
          <InputNumber
            min={1}
            precision={0}
            style={{ width: '50%' }}
            value={state.num}
            onChange={onNumber((v) => update({ num: Math.max(1, v) }))}
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
