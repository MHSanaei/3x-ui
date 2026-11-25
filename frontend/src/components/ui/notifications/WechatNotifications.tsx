import { InputNumber } from 'antd';
import {
  CloudServerOutlined,
  ThunderboltOutlined,
  DesktopOutlined,
  DashboardOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import type { AllSetting } from '@/models/setting';
import { onNumber } from '@/utils/onNumber';
import { NotificationLayout } from './NotificationLayout';
import { NotificationGroup } from './NotificationGroup';
import type { NotificationGroupConfig } from './types';

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
            precision={0}
            value={value}
            onChange={onNumber((next) => onChange(Math.round(next)))}
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
            precision={0}
            value={value}
            onChange={onNumber((next) => onChange(Math.round(next)))}
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
        key: 'cpu.high',
        label: 'eventCPUHigh',
        settingKey: 'wechatCpu',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber
            size="small"
            min={0}
            max={100}
            precision={0}
            value={value}
            onChange={onNumber((next) => onChange(Math.round(next)))}
            aria-label={ariaLabel}
            style={{ width: 80 }}
          />
        ),
      },
      {
        key: 'memory.high',
        label: 'eventMemoryHigh',
        settingKey: 'wechatMemory',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber
            size="small"
            min={0}
            max={100}
            precision={0}
            value={value}
            onChange={onNumber((next) => onChange(Math.round(next)))}
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

export function WechatNotifications({ allSetting, updateSetting }: Props) {
  const events = allSetting.wechatEnabledEvents || '';
  const selected = events
    ? events
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
    : [];

  function toggle(key: string) {
    const next = selected.includes(key) ? selected.filter((e) => e !== key) : [...selected, key];
    updateSetting({ wechatEnabledEvents: next.join(',') });
  }

  function toggleAll(keys: string[]) {
    const allSelected = keys.every((v) => selected.includes(v));
    const next = allSelected
      ? selected.filter((v) => !keys.includes(v))
      : [...new Set([...selected, ...keys])];
    updateSetting({ wechatEnabledEvents: next.join(',') });
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
    </NotificationLayout>
  );
}
