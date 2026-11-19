import { InputNumber, Space, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import {
  CloudServerOutlined,
  ThunderboltOutlined,
  DesktopOutlined,
  DashboardOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import type { AllSetting } from '@/models/setting';
import { NotificationLayout } from './NotificationLayout';
import { NotificationGroup } from './NotificationGroup';
import type { NotificationGroupConfig } from './types';
import { UserTrafficAlertInputs } from './UserTrafficAlertInputs';

const GROUPS: NotificationGroupConfig[] = [
  {
    icon: <CloudServerOutlined aria-hidden />,
    title: 'eventGroupOutbound',
    events: [
      {
        key: 'outbound.down',
        label: 'eventOutboundDown',
        settingKey: 'outboundDownThreshold',
        hint: 'eventDownThresholdHint',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber
            size="small"
            min={1}
            max={100}
            value={value}
            onChange={onChange}
            aria-label={ariaLabel}
            style={{ width: 80 }}
          />
        ),
      },
      { key: 'outbound.up', label: 'eventOutboundUp', settingKey: '' },
    ],
  },
  {
    icon: <ThunderboltOutlined aria-hidden />,
    title: 'eventGroupXray',
    events: [{ key: 'xray.crash', label: 'eventXrayCrash', settingKey: '' }],
  },
  {
    icon: <DesktopOutlined aria-hidden />,
    title: 'eventGroupNode',
    events: [
      {
        key: 'node.down',
        label: 'eventNodeDown',
        settingKey: 'nodeDownThreshold',
        hint: 'eventDownThresholdHint',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber
            size="small"
            min={1}
            max={100}
            value={value}
            onChange={onChange}
            aria-label={ariaLabel}
            style={{ width: 80 }}
          />
        ),
      },
      { key: 'node.up', label: 'eventNodeUp', settingKey: '' },
    ],
  },
  {
    icon: <DashboardOutlined aria-hidden />,
    title: 'eventGroupSystem',
    events: [
      {
        key: 'user.rate.high',
        label: 'eventUserRateHigh',
        settingKey: 'smtpUserRateMB',
        extra: ({ allSetting, updateSetting }) => (
          <UserTrafficAlertInputs
            rule="rate"
            allSetting={allSetting}
            updateSetting={updateSetting}
          />
        ),
      },
      {
        key: 'user.traffic.high',
        label: 'eventUserTrafficHigh',
        settingKey: 'smtpUserWindowGB',
        extra: ({ allSetting, updateSetting }) => (
          <UserTrafficAlertInputs
            rule="window"
            allSetting={allSetting}
            updateSetting={updateSetting}
          />
        ),
      },
      {
        key: 'cpu.high',
        label: 'eventCPUHigh',
        settingKey: 'smtpCpu',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber
            size="small"
            min={0}
            max={100}
            value={value}
            onChange={onChange}
            aria-label={ariaLabel}
            style={{ width: 80 }}
          />
        ),
      },
      {
        key: 'memory.high',
        label: 'eventMemoryHigh',
        settingKey: 'smtpMemory',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber
            size="small"
            min={0}
            max={100}
            value={value}
            onChange={onChange}
            aria-label={ariaLabel}
            style={{ width: 80 }}
          />
        ),
      },
    ],
  },
  {
    icon: <SafetyOutlined aria-hidden />,
    title: 'eventGroupSecurity',
    events: [{ key: 'login.attempt', label: 'eventLoginAttempt', settingKey: '' }],
  },
];

interface Props {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export function EmailNotifications({ allSetting, updateSetting }: Props) {
  const { t } = useTranslation();
  const events = allSetting.smtpEnabledEvents || '';
  const selected = events
    ? events
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
    : [];

  function toggle(key: string) {
    const next = selected.includes(key) ? selected.filter((e) => e !== key) : [...selected, key];
    updateSetting({ smtpEnabledEvents: next.join(',') });
  }

  function toggleAll(keys: string[]) {
    const allSelected = keys.every((v) => selected.includes(v));
    const next = allSelected
      ? selected.filter((v) => !keys.includes(v))
      : [...new Set([...selected, ...keys])];
    updateSetting({ smtpEnabledEvents: next.join(',') });
  }

  return (
    <NotificationLayout>
      {GROUPS.map((group, i) => (
        <NotificationGroup
          key={i}
          config={group}
          selected={selected}
          onToggle={toggle}
          onToggleAll={toggleAll}
          allSetting={allSetting}
          updateSetting={updateSetting}
        />
      ))}
      {selected.some((key) => key === 'user.rate.high' || key === 'user.traffic.high') && (
        <Space orientation="vertical" size={8} style={{ gridColumn: '1 / -1' }}>
          <Space wrap>
            <Typography.Text>{t('pages.settings.userTrafficCooldown')}</Typography.Text>
            <InputNumber
              min={1}
              max={10080}
              precision={0}
              value={allSetting.smtpUserCooldownMinutes}
              aria-label={t('pages.settings.userTrafficCooldown')}
              onChange={(value) => {
                if (value != null) updateSetting({ smtpUserCooldownMinutes: value });
              }}
            />
            <Typography.Text>{t('pages.settings.userTrafficMinutes')}</Typography.Text>
          </Space>
          <Typography.Paragraph type="secondary">
            {t('pages.settings.userTrafficHelp')}
          </Typography.Paragraph>
        </Space>
      )}
    </NotificationLayout>
  );
}
