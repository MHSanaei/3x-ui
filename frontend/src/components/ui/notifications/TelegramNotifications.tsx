import { InputNumber } from 'antd';
import { CloudServerOutlined, ThunderboltOutlined, DesktopOutlined, DashboardOutlined, SafetyOutlined } from '@ant-design/icons';
import type { AllSetting } from '@/models/setting';
import { NotificationLayout } from './NotificationLayout';
import { NotificationGroup } from './NotificationGroup';
import type { NotificationGroupConfig } from './types';

const GROUPS: NotificationGroupConfig[] = [
  {
    icon: <CloudServerOutlined />,
    title: 'eventGroupOutbound',
    events: [
      {
        key: 'outbound.down',
        label: 'eventOutboundDown',
        settingKey: 'outboundDownThreshold',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber size="small" min={1} max={100} value={value} onChange={onChange} aria-label={ariaLabel} style={{ width: 80 }} />
        ),
      },
      { key: 'outbound.up', label: 'eventOutboundUp', settingKey: '' },
    ],
  },
  {
    icon: <ThunderboltOutlined />,
    title: 'eventGroupXray',
    events: [
      { key: 'xray.crash', label: 'eventXrayCrash', settingKey: '' },
    ],
  },
  {
    icon: <DesktopOutlined />,
    title: 'eventGroupNode',
    events: [
      { key: 'node.down', label: 'eventNodeDown', settingKey: '' },
      { key: 'node.up', label: 'eventNodeUp', settingKey: '' },
    ],
  },
  {
    icon: <DashboardOutlined />,
    title: 'eventGroupSystem',
    events: [
      {
        key: 'cpu.high',
        label: 'eventCPUHigh',
        settingKey: 'tgCpu',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber size="small" min={0} max={100} value={value} onChange={onChange} aria-label={ariaLabel} style={{ width: 80 }} />
        ),
      },
      {
        key: 'memory.high',
        label: 'eventMemoryHigh',
        settingKey: 'tgMemory',
        extra: ({ value, onChange, ariaLabel }) => (
          <InputNumber size="small" min={0} max={100} value={value} onChange={onChange} aria-label={ariaLabel} style={{ width: 80 }} />
        ),
      },
    ],
  },
  {
    icon: <SafetyOutlined />,
    title: 'eventGroupSecurity',
    events: [
      { key: 'login.attempt', label: 'eventLoginAttempt', settingKey: '' },
    ],
  },
];

interface Props {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export function TelegramNotifications({ allSetting, updateSetting }: Props) {
  const events = allSetting.tgEnabledEvents || '';
  const selected = events ? events.split(',').map((s) => s.trim()).filter(Boolean) : [];

  function toggle(key: string) {
    const next = selected.includes(key)
      ? selected.filter((e) => e !== key)
      : [...selected, key];
    updateSetting({ tgEnabledEvents: next.join(',') });
  }

  function toggleAll(keys: string[]) {
    const allSelected = keys.every((v) => selected.includes(v));
    const next = allSelected
      ? selected.filter((v) => !keys.includes(v))
      : [...new Set([...selected, ...keys])];
    updateSetting({ tgEnabledEvents: next.join(',') });
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
