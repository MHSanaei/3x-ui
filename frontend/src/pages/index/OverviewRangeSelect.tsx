import { Select } from 'antd';
import { useTranslation } from 'react-i18next';

export const OVERVIEW_RANGES = [2, 5, 30, 60, 360, 720] as const;
export type OverviewRange = (typeof OVERVIEW_RANGES)[number];

interface OverviewRangeSelectProps {
  value: OverviewRange;
  onChange: (value: OverviewRange) => void;
}

const OPTIONS = [
  { value: 2, label: '2m' },
  { value: 5, label: '5m' },
  { value: 30, label: '30m' },
  { value: 60, label: '1h' },
  { value: 360, label: '6h' },
  { value: 720, label: '12h' },
];

export default function OverviewRangeSelect({ value, onChange }: OverviewRangeSelectProps) {
  const { t } = useTranslation();

  return (
    <Select
      aria-label={t('pages.index.timeRange')}
      className="ov-range-select"
      size="small"
      value={value}
      options={OPTIONS}
      onChange={(next: number) => onChange(next as OverviewRange)}
    />
  );
}
