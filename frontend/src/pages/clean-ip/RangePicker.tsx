import { useTranslation } from 'react-i18next';
import { Checkbox, Collapse, Input, Typography } from 'antd';

import type { CleanIpCdn } from './scan';

export default function RangePicker({
  cdn,
  ticked,
  onTicked,
  extra,
  onExtra,
  disabled,
}: {
  cdn: CleanIpCdn;
  ticked: string[];
  onTicked: (ranges: string[]) => void;
  extra: string;
  onExtra: (text: string) => void;
  disabled: boolean;
}) {
  const { t } = useTranslation();
  const on = new Set(ticked);
  const setGroup = (ranges: string[], value: boolean) => {
    const next = new Set(ticked);
    for (const r of ranges) {
      if (value) next.add(r);
      else next.delete(r);
    }
    onTicked([...next]);
  };

  return (
    <div className="clean-ip-ranges">
      <Collapse
        size="small"
        items={cdn.groups.map((g) => {
          const n = g.ranges.filter((r) => on.has(r)).length;
          return {
            key: g.id,
            label: (
              <Checkbox
                checked={n === g.ranges.length}
                indeterminate={n > 0 && n < g.ranges.length}
                disabled={disabled}
                onClick={(e) => e.stopPropagation()}
                onChange={(e) => setGroup(g.ranges, e.target.checked)}
              >
                {t(`pages.cleanIp.rangeGroups.${g.id}`)}{' '}
                <Typography.Text type="secondary">
                  ({n}/{g.ranges.length})
                </Typography.Text>
              </Checkbox>
            ),
            children: (
              <Checkbox.Group
                className="clean-ip-range-list clean-ip-mono"
                disabled={disabled}
                value={g.ranges.filter((r) => on.has(r))}
                options={g.ranges.map((r) => ({ label: r, value: r }))}
                onChange={(values) => {
                  const keep = ticked.filter((r) => !g.ranges.includes(r));
                  onTicked([...keep, ...(values as string[])]);
                }}
              />
            ),
          };
        })}
      />
      <label className="clean-ip-field clean-ip-note">
        <span>{t('pages.cleanIp.extraRanges')}</span>
        <Input.TextArea
          className="clean-ip-mono"
          rows={2}
          value={extra}
          disabled={disabled}
          placeholder="104.16.0.0/20"
          onChange={(e) => onExtra(e.target.value)}
        />
      </label>
    </div>
  );
}
