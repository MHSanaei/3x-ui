import { Checkbox, Collapse, InputNumber, Space } from 'antd';
import { DownOutlined, RightOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

interface EventGroup {
  key: string;
  labelKey: string;
  events: { value: string; labelKey: string }[];
}

const EVENT_GROUPS: EventGroup[] = [
  {
    key: 'outbound',
    labelKey: 'pages.settings.eventGroupOutbound',
    events: [
      { value: 'outbound.down', labelKey: 'pages.settings.eventOutboundDown' },
      { value: 'outbound.up', labelKey: 'pages.settings.eventOutboundUp' },
    ],
  },
  {
    key: 'xray',
    labelKey: 'pages.settings.eventGroupXray',
    events: [
      { value: 'xray.crash', labelKey: 'pages.settings.eventXrayCrash' },
    ],
  },
  {
    key: 'node',
    labelKey: 'pages.settings.eventGroupNode',
    events: [
      { value: 'node.down', labelKey: 'pages.settings.eventNodeDown' },
      { value: 'node.up', labelKey: 'pages.settings.eventNodeUp' },
    ],
  },
  {
    key: 'system',
    labelKey: 'pages.settings.eventGroupSystem',
    events: [
      { value: 'cpu.high', labelKey: 'pages.settings.eventCPUHigh' },
    ],
  },
  {
    key: 'security',
    labelKey: 'pages.settings.eventGroupSecurity',
    events: [
      { value: 'login.attempt', labelKey: 'pages.settings.eventLoginAttempt' },
    ],
  },
];

interface EventBusCheckboxesProps {
  value: string;
  onChange: (v: string) => void;
  /** Maps event value → { key: setting field name, value: current value } for inline inputs */
  extra?: Record<string, { key: string; value: number }>;
  /** Callback when extra input changes: (settingKey, newValue) => void */
  onExtraChange?: (key: string, v: number | null) => void;
}

export function EventBusCheckboxes({ value, onChange, extra, onExtraChange }: EventBusCheckboxesProps) {
  const { t } = useTranslation();
  const selected = value ? value.split(',').map((s) => s.trim()).filter(Boolean) : [];

  function toggle(eventType: string) {
    const next = selected.includes(eventType)
      ? selected.filter((e) => e !== eventType)
      : [...selected, eventType];
    onChange(next.join(','));
  }

  function toggleGroup(group: EventGroup) {
    const groupValues = group.events.map((e) => e.value);
    const allSelected = groupValues.every((v) => selected.includes(v));
    let next: string[];
    if (allSelected) {
      next = selected.filter((v) => !groupValues.includes(v));
    } else {
      next = [...new Set([...selected, ...groupValues])];
    }
    onChange(next.join(','));
  }

  const items = EVENT_GROUPS.map((group) => {
    const count = group.events.filter((e) => selected.includes(e.value)).length;
    const total = group.events.length;
    const allSelected = count === total;

    return {
      key: group.key,
      label: (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontWeight: 500 }}>{t(group.labelKey)}</span>
          <span style={{ color: '#999', fontSize: 12 }}>
            {count}/{total}
          </span>
          <Checkbox
            checked={allSelected}
            indeterminate={count > 0 && count < total}
            onClick={(e) => e.stopPropagation()}
            onChange={() => toggleGroup(group)}
          />
        </div>
      ),
      children: (
        <Checkbox.Group value={selected} style={{ width: '100%' }}>
          <Space wrap size={[16, 4]}>
            {group.events.map((et) => {
              const checked = selected.includes(et.value);
              const extraConf = extra?.[et.value];
              return (
                <span key={et.value} style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                  <Checkbox value={et.value} onChange={() => toggle(et.value)}>
                    {t(et.labelKey)}
                  </Checkbox>
                  {extraConf && onExtraChange && (
                    <InputNumber
                      size="small"
                      min={0}
                      max={100}
                      value={extraConf.value}
                      disabled={!checked}
                      onChange={(v) => onExtraChange(extraConf.key, v)}
                      style={{ width: 60 }}
                    />
                  )}
                </span>
              );
            })}
          </Space>
        </Checkbox.Group>
      ),
    };
  });

  const defaultActiveKeys = EVENT_GROUPS
    .filter((g) => g.events.some((e) => selected.includes(e.value)))
    .map((g) => g.key);

  return (
    <Collapse
      items={items}
      defaultActiveKey={defaultActiveKeys.length > 0 ? defaultActiveKeys : ['outbound']}
      expandIcon={({ isActive }) => isActive ? <DownOutlined /> : <RightOutlined />}
      size="small"
    />
  );
}
